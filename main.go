package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gin-gonic/gin"
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

func main() {
	flag.Parse()
	gin.SetMode(gin.ReleaseMode)

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
	client := s3.New(auth, aws.USWest) // TODO: make the region configurable

	// Authorized accounts
	accounts := gin.Accounts{}
	accounts[config.Auth.Username] = config.Auth.Password

	r := gin.New()
	r.Use(RequestIdMiddleware)
	r.Use(LogrusMiddleware)

	// Inject our config and S3 instance into each request.
	r.Use(func(c *gin.Context) {
		c.Set("client", client)
		c.Set("config", &config)
		c.Next()
	})

	// Handle errors by writing them as JSON.
	r.Use(ErrorPrintMiddleware)

	r.GET("/", Index)

	authorized := r.Group("/", gin.BasicAuth(accounts))
	{
		authorized.POST("/upload", Upload)
	}

	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("Starting HTTP server on %s", addr)
	r.Run(":8080")
}
