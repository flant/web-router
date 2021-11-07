package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

// Get some status info
func statusHandler(w http.ResponseWriter, r *http.Request) {
	var msg []string
	status := "ok"

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	err := updateReleasesStatus()
	if err != nil {
		msg = append(msg, err.Error())
		status = "error"
	}

	_ = json.NewEncoder(w).Encode(
		APIStatusResponseType{
			Status:         status,
			Msg:            strings.Join(msg, " "),
			RootVersion:    getRootReleaseVersion(),
			RootVersionURL: VersionToURL(getRootReleaseVersion()),
			Releases:       ReleasesStatus.Groups,
		})
}

// X-Redirect to the stablest documentation version for specific group
func groupHandler(w http.ResponseWriter, r *http.Request) {
	var langPrefix string

	_ = updateReleasesStatus()
	log.Debugln("Use handler - groupHandler")

	vars := mux.Vars(r)
	if len(vars["lang"]) > 0 {
		langPrefix = fmt.Sprintf("/%s", vars["lang"])
	}

	if version, err := getVersionFromGroup(&ReleasesStatus, vars["group"]); err == nil {
		w.Header().Set("X-Accel-Redirect", fmt.Sprintf("%s%s/%s/%s", langPrefix, GlobalConfig.LocationVersions, VersionToURL(version), getDocPageURLRelative(r, true)))
	} else {
		http.Redirect(w, r, fmt.Sprintf("%s%s/%s/", langPrefix, GlobalConfig.LocationVersions, GlobalConfig.DefaultGroup), 302)
	}
}

// Handles request to /v<group>-<channel>/. E.g. /v1.2-beta/
// Temprarily redirect to specific version
func groupChannelHandler(w http.ResponseWriter, r *http.Request) {
	var version, URLToRedirect, langPrefix string
	var err error

	log.Debugln("Use handler - groupChannelHandler")

	pageURLRelative := "/"
	vars := mux.Vars(r)
	if len(vars["lang"]) > 0 {
		langPrefix = fmt.Sprintf("/%s", vars["lang"])
	}

	_ = updateReleasesStatus()

	re := regexp.MustCompile(fmt.Sprintf("^/(ru|en)%s/[^/]+/(.+)$", GlobalConfig.LocationVersions))
	res := re.FindStringSubmatch(r.URL.RequestURI())
	if res != nil {
		pageURLRelative = res[2]
	}

	version, err = getVersionFromChannelAndGroup(&ReleasesStatus, vars["channel"], vars["group"])
	if err == nil {
		URLToRedirect = fmt.Sprintf("%s%s/%s/%s", langPrefix, GlobalConfig.LocationVersions, VersionToURL(version), pageURLRelative)
		err = validateURL(fmt.Sprintf("https://%s%s", r.Host, URLToRedirect))
	}

	if err != nil {
		log.Errorf("Error validating URL: %v, (original was https://%s/%s)", err.Error(), r.Host, r.URL.RequestURI())
		notFoundHandler(w, r)
	} else {
		http.Redirect(w, r, URLToRedirect, 302)
	}
}

// Healthcheck handler
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Render templates
func templateHandler(w http.ResponseWriter, r *http.Request) {
	if err := updateReleasesStatus(); err != nil {
		log.Println(err)
	}

	templateData := templateDataType{
		VersionItems:           []versionMenuItems{},
		CurrentGroup:           "", // not used now
		CurrentChannel:         "",
		CurrentVersion:         "",
		CurrentLang:            "",
		AbsoluteVersion:        "",
		CurrentVersionURL:      "",
		CurrentPageURLRelative: "",
		CurrentPageURL:         "",
		MenuDocumentationLink:  "",
	}

	_ = templateData.getVersionMenuData(r)

	tplPath := getRootFilesPath() + r.URL.Path
	tpl := template.Must(template.ParseFiles(tplPath))
	err := tpl.Execute(w, templateData)
	if err != nil {
		// Should we do some magic here or can simply log error?
		log.Errorf("Internal Server Error (template error), %s ", err.Error())
		http.Error(w, "Internal Server Error (template error)", 500)
	}
}

func serveFilesHandler(fs http.FileSystem) http.Handler {
	fsh := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			r.URL.Path = upath
		}
		upath = path.Clean(upath)
		if _, err := os.Stat(fmt.Sprintf("%v%s", fs, upath)); err != nil {
			if os.IsNotExist(err) {
				notFoundHandler(w, r)
				return
			}
		}
		fsh.ServeHTTP(w, r)
	})
}

func rootDocHandler(w http.ResponseWriter, r *http.Request) {
	var redirectTo, langPrefix string

	log.Debugln("Use handler - rootDocHandler")

	vars := mux.Vars(r)
	if len(vars["lang"]) > 0 {
		langPrefix = fmt.Sprintf("/%s", vars["lang"])
	}

	if hasSuffix, _ := regexp.MatchString(fmt.Sprintf("^/[^/]+%s/.+", GlobalConfig.LocationVersions), r.RequestURI); hasSuffix {
		items := strings.Split(r.RequestURI, fmt.Sprintf("%s/", GlobalConfig.LocationVersions))
		if len(items) > 1 {
			redirectTo = strings.Join(items[1:], fmt.Sprintf("%s%s/", langPrefix, GlobalConfig.LocationVersions))
		}
	}

	http.Redirect(w, r, fmt.Sprintf("%s%s/%s/%s", langPrefix, GlobalConfig.LocationVersions, GlobalConfig.DefaultGroup, redirectTo), 301)
}

// Redirect to root documentation if request not matches any location (override 404 response)
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	lang := "en"

	re := regexp.MustCompile(`^/(ru|en)/.*$`)
	res := re.FindStringSubmatch(r.URL.RequestURI())
	if res != nil {
		lang = res[1]
	}

	w.WriteHeader(http.StatusNotFound)
	page404File, err := os.Open(fmt.Sprintf("%s/%s/404.html", getRootFilesPath(), lang))
	defer page404File.Close()
	if err != nil {
		// 404.html file not found! Send the fallback page...
		log.Error("404.html file not found")
		http.Error(w, `<html lang="en">
<head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <title>Page Not Found</title>
    <meta name="title" content="Page Not Found">
</head>
<body style="
    display: flex;
    flex-direction: column;
    height: -webkit-fill-available;
    justify-content: space-between;
">
<div class="content">
    <div style="margin-top: 100px; width: 100%; width: 80%; margin-left: 50px;">
        <h1 class="docs__title">Page not found</h1>
        <div class="post-content">
            <p>Sorry, the page you were looking for does not exist.</p>
            <p>Try searching for it or check the URL to see if it looks correct.</p>
        </div>
    </div>
</div>
</body>
</html>`, 404)
		return
	}
	io.Copy(w, page404File)
}
