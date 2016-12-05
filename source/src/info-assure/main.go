package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/contrib/renders/multitemplate"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/voxelbrain/goptions"
	util "github.com/woanware/goutil"
	"gopkg.in/mgutz/dat.v1"
	"gopkg.in/mgutz/dat.v1/sqlx-runner"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ##### Variables ###########################################################

var (
	logger *logging.Logger
	config *Config
	db     *runner.DB
	users  map[string]User
)

// ##### Constants ###########################################################

const APP_TITLE string = "AutoRun Logger UI"
const APP_NAME = "arl-ui"
const APP_VERSION = "1.0.3"

// ##### Methods #############################################################

func main() {
	fmt.Printf("\n%s (%s) %s\n\n", APP_TITLE, APP_NAME, APP_VERSION)

	initialiseLogging()

	opt := struct {
		ConfigFile string        `goptions:"-c, --config, description='Config file path'"`
		UsersFile  string        `goptions:"-u, --users, description='Users file path'"`
		Help       goptions.Help `goptions:"-h, --help, description='Show this help'"`
	}{ // Default values
		ConfigFile: "./" + APP_NAME + ".config",
		UsersFile:  "./users.config",
	}

	goptions.ParseAndFail(&opt)

	loadConfig(opt.ConfigFile)
	loadUsers(opt.UsersFile)

	initialiseDatabase()
	setupHttpServer()
}

func setupHttpServer() {

	// Define the user accounts in a username:password map that
	// can be used with the BasicAuth gin-gonic middleware
	tmpAccounts := make(gin.Accounts)
	for _, u := range users {
		tmpAccounts[u.UserName] = u.Password
		if len(u.Password) == 0 {
			logger.Fatalf("User configured without a password defined: %s", u.UserName)
		}
	}

	logger.Info("HTTP API server running: " + config.HttpIp + ":" + fmt.Sprintf("%d", config.HttpPort))
	var r *gin.Engine
	if config.Debug == true {
		r = gin.Default()
	} else {
		gin.SetMode(gin.ReleaseMode)
		r = gin.New()

		r.Use(gin.Recovery())
	}

	r.HTMLRender = loadTemplates(config.TemplateDir)
	r.Static("/static", config.StaticDir)

	// Group using gin.BasicAuth() middleware
	// gin.Accounts is a shortcut for map[string]string
	authorized := r.Group("/", gin.BasicAuth(tmpAccounts))

	authorized.GET("/", routeIndex)
	authorized.GET("/alerts", routeAlerts)
	authorized.POST("/alerts", routeAlerts)
	authorized.GET("/classified", routeClassified)
	authorized.POST("/classified", routeClassified)
	authorized.GET("/singlehost", routeSingleHost)
	authorized.POST("/singlehost", routeSingleHost)
	authorized.GET("/search", routeSearch)
	authorized.POST("/search", routeSearch)
	authorized.GET("/export", routeExport)
	authorized.POST("/export", routeExport)
	authorized.GET("/export/:id", routeExportData) // Download

	r.Run(config.HttpIp + ":" + fmt.Sprintf("%d", config.HttpPort))
}

func initialiseDatabase() {
	// create a normal database connection through database/sql
	tempDb, err := sql.Open("postgres",
		fmt.Sprintf("host=%s dbname=%s user=%s password=%s sslmode=disable",
			config.DatabaseServer, config.DatabaseName, config.DatabaseUser, config.DatabasePassword))

	if err != nil {
		logger.Fatalf("Unable to open database connection: %v", err)
		return
	}

	// ensures the database can be pinged with an exponential backoff (15 min)
	runner.MustPing(tempDb)

	// set to reasonable values for production
	tempDb.SetMaxIdleConns(4)
	tempDb.SetMaxOpenConns(16)

	// set this to enable interpolation
	dat.EnableInterpolation = true

	// set to check things like sessions closing.
	// Should be disabled in production/release builds.
	dat.Strict = false

	// Log any query over 10ms as warnings. (optional)
	runner.LogQueriesThreshold = 50 * time.Millisecond

	db = runner.NewDB(tempDb, "postgres")
}

// Loads the config file contents (yaml) and marshals to a struct
func loadConfig(configPath string) {
	config = new(Config)
	data, err := util.ReadTextFromFile(configPath)
	if err != nil {
		logger.Fatalf("Error reading the config file: %v", err)
	}

	err = yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		logger.Fatalf("Error unmarshalling the config file: %v", err)
	}

	if len(config.DatabaseServer) == 0 {
		logger.Fatal("Database server not set in config file")
	}

	if len(config.DatabaseName) == 0 {
		logger.Fatal("Database name not set in config file")
	}

	if len(config.DatabaseUser) == 0 {
		logger.Fatal("Database user not set in config file")
	}

	if len(config.DatabasePassword) == 0 {
		logger.Fatal("Database password not set in config file")
	}

	if len(config.HttpIp) == 0 {
		config.HttpIp = "127.0.0.1"
	}

	if config.HttpPort == 0 {
		config.HttpPort = 8080
	}

	if len(config.StaticDir) == 0 {
		logger.Fatal("Static directory not set in config file")
	}

	if len(config.TemplateDir) == 0 {
		logger.Fatal("Template directory not set in config file")
	}

	if len(config.ExportDir) == 0 {
		logger.Fatal("Export dir not set in config file")
	}
}

// Loads the config file contents (yaml) and marshals to a struct
func loadUsers(configPath string) {
	temp := new(Users)
	data, err := util.ReadTextFromFile(configPath)
	if err != nil {
		log.Fatalf("Error reading the users config file: %v", err)
	}

	err = yaml.Unmarshal([]byte(data), &temp)
	if err != nil {
		log.Fatalf("Error unmarshalling the users config file: %v", err)
	}

	users = make(map[string]User)
	for _, u := range temp.Data {
		users[u.UserName] = u
	}
}

// Sets up the logging infrastructure e.g. Stdout and /var/log
func initialiseLogging() {
	// Setup the actual loggers
	logger = logging.MustGetLogger(APP_NAME)

	// Check that we have a "nca" sub directory in /var/log
	if _, err := os.Stat("/var/log/" + APP_NAME); os.IsNotExist(err) {
		logger.Fatalf("The /var/log/%s directory does not exist", APP_NAME)
	}

	// Check that we have permission to write to the /var/log/APP_NAME directory
	f, err := os.Create("/var/log/" + APP_NAME + "/test.txt")
	if err != nil {
		logger.Fatalf("Unable to write to /var/log/%s", APP_NAME)
	}

	// Clear up our tests
	os.Remove("/var/log/" + APP_NAME + "/test.txt")
	f.Close()

	// Define the /var/log file
	logFile, err := os.OpenFile("/var/log/"+APP_NAME+"/log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Fatalf("Error opening the log file: %v", err)
	}

	// Define the StdOut logging
	backendStdOut := logging.NewLogBackend(os.Stdout, "", 0)
	formatStdOut := logging.MustStringFormatter(
		"%{color}%{time:2006-01-02T15:04:05.000} %{color:reset} %{message}")
	formatterStdOut := logging.NewBackendFormatter(backendStdOut, formatStdOut)

	// Define the /var/log logging
	backendFile := logging.NewLogBackend(logFile, "", 0)
	formatFile := logging.MustStringFormatter(
		"%{time:2006-01-02T15:04:05.000} %{level:.4s} %{message}")
	formatterFile := logging.NewBackendFormatter(backendFile, formatFile)

	logging.SetBackend(formatterStdOut, formatterFile)
}

// Loads the templates for each of the pages
func loadTemplates(templatesDir string) multitemplate.Render {

	r := multitemplate.New()

	r.AddFromFiles("index",
		filepath.Join(templatesDir, "base.tmpl"), filepath.Join(templatesDir, "index.tmpl"))
	r.AddFromFiles("alerts",
		filepath.Join(templatesDir, "base.tmpl"), filepath.Join(templatesDir, "alerts.tmpl"),
		filepath.Join(templatesDir, "buttons.tmpl"), filepath.Join(templatesDir, "alerts_table.tmpl"))
	r.AddFromFiles("classified",
		filepath.Join(templatesDir, "base.tmpl"), filepath.Join(templatesDir, "classified.tmpl"),
		filepath.Join(templatesDir, "classified_buttons.tmpl"), filepath.Join(templatesDir, "classified_table.tmpl"))
	r.AddFromFiles("single_host",
		filepath.Join(templatesDir, "base.tmpl"), filepath.Join(templatesDir, "single_host.tmpl"),
		filepath.Join(templatesDir, "buttons.tmpl"), filepath.Join(templatesDir, "single_host_table.tmpl"))
	r.AddFromFiles("export",
		filepath.Join(templatesDir, "base.tmpl"), filepath.Join(templatesDir, "export.tmpl"),
		filepath.Join(templatesDir, "export_table.tmpl"))
	r.AddFromFiles("search",
		filepath.Join(templatesDir, "base.tmpl"), filepath.Join(templatesDir, "search.tmpl"),
		filepath.Join(templatesDir, "buttons.tmpl"), filepath.Join(templatesDir, "search_table.tmpl"))

	return r
}
