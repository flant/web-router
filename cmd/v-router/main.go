package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var GlobalConfig GlobalConfigType
var channelList string

func newRouter() *mux.Router {
	var langPrefix string
	r := mux.NewRouter()

	staticFileDirectory := http.Dir(getRootFilesPath())

	if GlobalConfig.I18nType == "location" {
		langPrefix = "/{lang:ru|en}"
	}

	channelList = "alpha|beta|ea|stable|rock-solid"
	if GlobalConfig.ShowLatestChannel {
		channelList = "latest|" + channelList
	}

	r.PathPrefix("/status").HandlerFunc(statusHandler)
	r.PathPrefix("/health").HandlerFunc(healthCheckHandler)

	r.PathPrefix(fmt.Sprintf("%s%s/{group:v[0-9]+.[0-9]+}-{channel:%s}/", langPrefix, GlobalConfig.LocationVersions, channelList)).HandlerFunc(groupChannelHandler)
	r.PathPrefix(fmt.Sprintf("%s%s/{group:v[0-9]+}-{channel:%s}/", langPrefix, GlobalConfig.LocationVersions, channelList)).HandlerFunc(groupChannelHandler)
	r.PathPrefix(fmt.Sprintf("%s%s/{group:v[0-9]+}/", langPrefix, GlobalConfig.LocationVersions)).HandlerFunc(groupHandler)
	r.PathPrefix(fmt.Sprintf("%s%s/", langPrefix, GlobalConfig.LocationVersions)).HandlerFunc(rootDocHandler)
	r.HandleFunc(fmt.Sprintf("%s%s", langPrefix, GlobalConfig.LocationVersions), rootDocHandler)
	r.PathPrefix(fmt.Sprintf("%s%s/", langPrefix, GlobalConfig.PathTpls)).HandlerFunc(templateHandler)
	r.PathPrefix(fmt.Sprintf("%s/", GlobalConfig.PathTpls)).HandlerFunc(templateHandler)

	r.Path("/404.html").HandlerFunc(notFoundHandler)

	r.PathPrefix("/").Handler(serveFilesHandler(staticFileDirectory))

	r.Use(LoggingMiddleware)

	r.NotFoundHandler = r.NewRoute().HandlerFunc(notFoundHandler).GetHandler()

	return r
}

func main() {
	err := envconfig.Process("VROUTER", &GlobalConfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	Setup()
	ValidateConfig()
	printConfiguration()

	r := newRouter()

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("%s:%s", GlobalConfig.ListenAddress, GlobalConfig.ListenPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		if err != nil {
			log.Errorln(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown failed:%+s", err)
	}
	log.Infoln("Shutting down...")
}
