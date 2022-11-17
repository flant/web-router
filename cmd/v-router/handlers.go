package main

import (
	"encoding/json"
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
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
	if len(vars["lang"]) > 0 && GlobalConfig.I18nType == "location" {
		langPrefix = fmt.Sprintf("/%s", vars["lang"])
	}

	if version, err := getVersionFromGroup(&ReleasesStatus, vars["group"]); err == nil {
		log.Debugln(fmt.Sprintf("getVersionFromGroup: Got version - %s for x-redirect", version))
		w.Header().Set("X-Accel-Redirect", fmt.Sprintf("%s%s/%s/%s", langPrefix, GlobalConfig.LocationVersions, VersionToURL(version), getDocPageURLRelative(r, true)))
	} else {
		log.Debugln(fmt.Sprintf("getVersionFromGroup: Got error %e", err))
		http.Redirect(w, r, fmt.Sprintf("%s/", langPrefix), 302)
	}
}

// Handles request to /v<group>-<channel>/. E.g. /v1.2-beta/
// Temporarily redirect to specific version
func groupChannelHandler(w http.ResponseWriter, r *http.Request) {
	var version, URLToRedirect, langPrefix string
	var re *regexp.Regexp
	var err error

	log.Debugln("Use handler - groupChannelHandler")

	pageURLRelative := "/"
	vars := mux.Vars(r)
	if len(vars["lang"]) > 0 && GlobalConfig.I18nType == "location" {
		langPrefix = fmt.Sprintf("/%s", vars["lang"])
	}

	_ = updateReleasesStatus()

	if GlobalConfig.I18nType == "location" {
		re = regexp.MustCompile(fmt.Sprintf("^/(ru|en)%s/[^/]+/(.+)$", GlobalConfig.LocationVersions))
		res := re.FindStringSubmatch(r.URL.RequestURI())
		if res != nil {
			pageURLRelative = res[2]
		}
	} else {
		re = regexp.MustCompile(fmt.Sprintf("^%s/[^/]+/(.+)$", GlobalConfig.LocationVersions))
		res := re.FindStringSubmatch(r.URL.RequestURI())
		if res != nil {
			pageURLRelative = res[1]
		}
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
	var tplPath string
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

	switch GlobalConfig.I18nType {
	case "location":
		tplPath = getRootFilesPath() + r.URL.Path
	case "separate-domain":
		language := getLanguageFromDomainMap(r.Host)
		log.Debugf("Detected %s language for the %s domain", language, r.Host)
		tplPath = fmt.Sprintf("%s/%s%s", getRootFilesPath(), language, r.URL.Path)
	case "domain":
		language := getLanguageFromDomain(r.Host)
		log.Debugf("Detected %s language for the %s domain", language, r.Host)
		tplPath = fmt.Sprintf("%s/%s%s", getRootFilesPath(), language, r.URL.Path)
	}
	templateContent, err := ioutil.ReadFile(tplPath)
	if err != nil {
		log.Errorf("Can't read the template file %s: %s ", tplPath, err.Error())
		http.Error(w, "<!-- Internal Server Error (template error) -->", 500)
	}

	tpl := template.Must(template.New("template").Funcs(sprig.FuncMap()).Parse(string(templateContent)))

	err = tpl.Execute(w, templateData)
	if err != nil {
		// Should we do some magic here or can simply log error?
		log.Errorf("Internal Server Error (template error), %s ", err.Error())
		http.Error(w, "<!-- Internal Server Error (template error) -->", 500)
	}
}

func getLanguageFromDomainMap(input string) string {
	result := ""
	host := strings.Split(input, ":")[0]
	for lang, domain := range DomainMap {
		// Get the first value from the map, to use as the default.
		if result == "" {
			result = lang
		}

		if host == domain || host == "www."+domain {
			return lang
		}
	}

	return result
}

func getLanguageFromDomain(input string) string {
	// Use en as the default language.
	result := "en"

	host := strings.Split(input, ":")[0]

	if strings.HasPrefix(host, "ru.") || strings.HasPrefix(host, "www.ru.") {
		result = "ru"
	} else {
		result = "en"
	}

	return result
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
		fileInfo, err := os.Stat(fmt.Sprintf("%v%s", fs, upath))

		if err != nil {
			if os.IsNotExist(err) {
				notFoundHandler(w, r)
				return
			}
		}

		if fileInfo.IsDir() {
			indexFile := filepath.Join(upath, "index.html")
			if _, err := os.Stat(fmt.Sprintf("%v%s", fs, indexFile)); err != nil {
				notFoundHandler(w, r)
				return
			}
		}
		log.Tracef("Serving file " + r.URL.Path)
		fsh.ServeHTTP(w, r)
	})
}

func rootDocHandler(w http.ResponseWriter, r *http.Request) {
	var redirectTo, langPrefix string

	log.Debugln("Use handler - rootDocHandler")

	vars := mux.Vars(r)
	if len(vars["lang"]) > 0 && GlobalConfig.I18nType == "location" {
		langPrefix = fmt.Sprintf("/%s", vars["lang"])
	}

	if hasSuffix, _ := regexp.MatchString(fmt.Sprintf("^/[^/]+%s/.+", GlobalConfig.LocationVersions), r.RequestURI); hasSuffix {
		items := strings.Split(r.RequestURI, fmt.Sprintf("%s/", GlobalConfig.LocationVersions))
		if len(items) > 1 {
			if isVersionOrChannel, _ := regexp.MatchString(fmt.Sprintf("^(%s|v[0-9]+.[0-9]+.[0-9]+([^/]+)?)[/]?", channelList), items[1]); isVersionOrChannel {
				// We can't handle requests to specific version. They should be routed by balancer (create corresponding Ingress resource)
				serveFilesHandler(http.Dir(getRootFilesPath())).ServeHTTP(w, r)
			}
			redirectTo = strings.Join(items[1:], fmt.Sprintf("%s%s/", langPrefix, GlobalConfig.LocationVersions))
		}
	}

	http.Redirect(w, r, fmt.Sprintf("%s%s/%s/%s", langPrefix, GlobalConfig.LocationVersions, GlobalConfig.DefaultGroup, redirectTo), 301)
}

// Redirect to root documentation if request not matches any location (override 404 response)
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	var re *regexp.Regexp
	lang := "en"

	switch GlobalConfig.I18nType {
	case "location":
		re = regexp.MustCompile(`^/(ru|en)/.*$`)
		res := re.FindStringSubmatch(r.URL.RequestURI())
		if res != nil {
			lang = res[1]
		}
	case "separate-domain":
		lang = getLanguageFromDomainMap(r.Host)
	case "domain":
		lang = getLanguageFromDomain(r.Host)
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
        <h1 class="docs__title">There was a glitch, try in a few seconds...</h1>
        <div class="post-content">
            <p>Sorry, maybe we are already investigating the problem and will fix it soon.</p>
        </div>
    </div>
</div>
</body>
</html>`, 404)
		return
	}
	io.Copy(w, page404File)
}
