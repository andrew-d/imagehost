package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
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
	gin.SetMode(gin.ReleaseMode)

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

	// Set up actual routes.
	r.GET("/", Index)

	authorized := r.Group("/", gin.BasicAuth(accounts))
	{
		authorized.POST("/upload", Upload)
	}

	// Good to go!
	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("Starting HTTP server on %s", addr)
	r.Run(":8080")
}
