package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
	flag "github.com/ogier/pflag"
	"gopkg.in/yaml.v1"
)

type Config struct {
	PublicBucket  string `yaml:"public_bucket"`
	ArchiveBucket string `yaml:"archive_bucket"`

	AWSAuth struct {
		AccessKey string `yaml:"access_key"`
		SecretKey string `yaml:"secret_key"`
		Token     string `yaml:"token"`
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

type routeParams struct {
	config *Config
	s3     *s3.S3
	params httprouter.Params
}

// Pseudo-middleware
type HandlerFunc func(w http.ResponseWriter, r *http.Request, p routeParams)

func wrapRoute(h HandlerFunc, template routeParams) httprouter.Handle {
	return httprouter.Handle(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		params := routeParams{
			config: template.config,
			s3:     template.s3,
			params: p,
		}

		h(w, r, params)
	})
}

func main() {
	flag.Parse()

	if len(flagConfigFile) == 0 {
		log.Printf("No config file specified")
		return
	}

	f, err := os.Open(flagConfigFile)
	if err != nil {
		log.Printf("Error opening config file: %s", err)
		return
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error reading config: %s", err)
		return
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Error decoding config: %s", err)
		return
	}

	// Validate config
	if len(config.PublicBucket) == 0 {
		log.Printf("No public bucket given")
		return
	}
	if len(config.AWSAuth.AccessKey) == 0 || len(config.AWSAuth.SecretKey) == 0 {
		log.Printf("AWS configuration not given")
		return
	}

	// Connect to AWS
	auth := aws.Auth{
		AccessKey: config.AWSAuth.AccessKey,
		SecretKey: config.AWSAuth.SecretKey,
		Token:     config.AWSAuth.Token,
	}
	s3 := s3.New(auth, aws.USWest) // TODO: make the region configurable

	// "Middleware"
	params := routeParams{
		config: &config,
		s3:     s3,
	}

	// Set up routes
	router := httprouter.New()
	router.GET("/", wrapRoute(Index, params))
	router.POST("/upload", wrapRoute(Upload, params))

	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("Starting HTTP server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
