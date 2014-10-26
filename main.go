package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goji/httpauth"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
	flag "github.com/ogier/pflag"
	"github.com/stretchr/graceful"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"
	"gopkg.in/yaml.v1"
)

type Config struct {
	PublicBucket  string `yaml:"public_bucket"`
	ArchiveBucket string `yaml:"archive_bucket"`

	AWSAuth struct {
		AccessKey string `yaml:"access_key"`
		SecretKey string `yaml:"secret_key"`
		Token     string `yaml:"token"`
		Region    string `yaml:"region"`
	} `yaml:"aws"`

	Auth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`
}

var (
	flagConfigFile string
	flagPort       int
)

func init() {
	flag.StringVarP(&flagConfigFile, "config", "c", "",
		"location of the config file")
	flag.IntVarP(&flagPort, "port", "p", 8080,
		"port to listen on")
}

func loadConfig(out *Config) error {
	if len(flagConfigFile) == 0 {
		return fmt.Errorf("No config file specified")
	}

	f, err := os.Open(flagConfigFile)
	if err != nil {
		return fmt.Errorf("Error opening config file: %s", err)
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("Error reading config: %s", err)
	}

	err = yaml.Unmarshal(data, out)
	if err != nil {
		return fmt.Errorf("Error decoding config: %s", err)
	}

	return nil
}

func validateConfig(config *Config) error {
	if len(config.PublicBucket) == 0 {
		return fmt.Errorf("No public bucket given")
	}
	if len(config.AWSAuth.AccessKey) == 0 || len(config.AWSAuth.SecretKey) == 0 {
		return fmt.Errorf("AWS configuration not given")
	}
	if len(config.AWSAuth.Region) > 0 {
		_, ok := aws.Regions[config.AWSAuth.Region]
		if !ok {
			return fmt.Errorf("AWS region '%s' not valid", config.AWSAuth.Region)
		}
	} else {
		config.AWSAuth.Region = "us-west-1"
	}

	return nil
}

func main() {
	flag.Parse()

	var config Config

	err := loadConfig(&config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":            err,
			"flagConfigFile": flagConfigFile,
		}).Error("Error loading config")
		return
	}

	err = validateConfig(&config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":            err,
			"flagConfigFile": flagConfigFile,
		}).Error("Error validating config")
		return
	}

	// Connect to AWS
	auth := aws.Auth{
		AccessKey: config.AWSAuth.AccessKey,
		SecretKey: config.AWSAuth.SecretKey,
		Token:     config.AWSAuth.Token,
	}
	client := s3.New(auth, aws.Regions[config.AWSAuth.Region])

	// Authorization
	authOpts := httpauth.AuthOptions{
		Realm:    "ImageHost",
		User:     config.Auth.Username,
		Password: config.Auth.Password,
	}

	m := web.New()
	m.Use(middleware.RequestID)
	m.Use(logMiddleware)
	m.Use(recoverMiddleware)
	m.Use(middleware.AutomaticOptions)

	// Inject our config and S3 instance into each request.
	m.Use(func(c *web.C, h http.Handler) http.Handler {
		ret := func(w http.ResponseWriter, r *http.Request) {
			c.Env["client"] = client
			c.Env["config"] = &config

			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(ret)
	})

	// Set up actual routes.
	m.Get("/", Index)

	authorized := web.New()
	authorized.Use(httpauth.BasicAuth(authOpts))
	authorized.Post("/upload", Upload)
	m.Handle("/*", authorized)

	// Good to go!
	addr := fmt.Sprintf(":%d", flagPort)
	log.Infof("Starting HTTP server on %s", addr)
	graceful.Run(addr, 10*time.Second, m)
	log.Infof("Finished")
}
