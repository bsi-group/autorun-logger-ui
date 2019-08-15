package main

import (
	"database/sql"
	"encoding/base32"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/contrib/renders/multitemplate"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/voxelbrain/goptions"
	util "github.com/woanware/goutil"
	"gopkg.in/mgutz/dat.v1"
	runner "gopkg.in/mgutz/dat.v1/sqlx-runner"
	"gopkg.in/yaml.v2"
)

// ##### Variables ###########################################################

var (
	logger       *logging.Logger
	config       *Config
	db           *runner.DB
	users        map[string]User
	sessionStore sessions.Store
)

// ##### Constants ###########################################################

const APP_TITLE string = "AutoRun Logger UI"

//const APP_NAME = "arl-ui"
const APP_NAME = "arl_web"
const APP_VERSION = "1.0.5"

// ##### Methods #############################################################

func main() {

	hash, _ := getPasswordHash("admin")
	log.Println(hash)

	// Generate random string for Google 2FA
	random := generateRandomString(16)
	// For Google Authenticator purpose: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
	secret := base32.StdEncoding.EncodeToString([]byte(random))
	log.Println(secret)

	//

	fmt.Printf("\n%s (%s) %s\n\n", APP_TITLE, APP_NAME, APP_VERSION)

	initialiseLogging()

	opt := struct {
		ConfigFile string        `goptions:"-c, --config, description='Config file path'"`
		Help       goptions.Help `goptions:"-h, --help, description='Show this help'"`
	}{ // Default values
		ConfigFile: "./" + APP_NAME + ".config",
	}

	goptions.ParseAndFail(&opt)

	loadConfig(opt.ConfigFile)

	initialiseDatabase()
	setupHttpServer()
}

//
func setupHttpServer() {

	logger.Info("HTTP API server running: " + config.HttpIp + ":" + fmt.Sprintf("%d", config.HttpPort))
	var router *gin.Engine
	if config.Debug == true {
		router = gin.Default()
	} else {
		gin.SetMode(gin.ReleaseMode)
		router = gin.New()

		router.Use(gin.Recovery())
	}

	sessionStore = cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions(APP_NAME, sessionStore))
	router.HTMLRender = loadTemplates("./templates")
	router.Static("/static", "./static")

	router.GET("/", routeLogonGet)
	router.POST("/", routeLogonPost)
	router.GET("/logout", routeLogout)

	authorized := router.Group("/")
	authorized.Use(AuthorizeMiddleware())
	{
		authorized.GET("/enroll", routeEnrollGet)
		authorized.POST("/enroll", routeEnrollPost)
		authorized.GET("/verify", routeVerifyGet)
		authorized.POST("/verify", routeVerifyPost)
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
		authorized.GET("/users", routeUsersGet)
		authorized.GET("/users/new", routeUserNewGet)
		authorized.POST("/users/new", routeUserNewPost)
	}

	router.Run(config.HttpIp + ":" + fmt.Sprintf("%d", config.HttpPort))
}

//
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

	// if len(config.StaticDir) == 0 {
	// 	logger.Fatal("Static directory not set in config file")
	// }

	// if len(config.TemplateDir) == 0 {
	// 	logger.Fatal("Template directory not set in config file")
	// }

	if len(config.ExportDir) == 0 {
		logger.Fatal("Export dir not set in config file")
	}
}

// Sets up the logging infrastructure e.g. Stdout and /var/log
func initialiseLogging() {

	// Setup the actual loggers
	logger = logging.MustGetLogger(APP_NAME)

	// Define the /var/log file
	logFile, err := os.OpenFile("./"+APP_NAME+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "index.html"))
	r.AddFromFiles("logon",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "logon.html"))
	r.AddFromFiles("enroll",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "enroll.html"))
	r.AddFromFiles("verify",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "verify.html"))
	r.AddFromFiles("users",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "users.html"))
	r.AddFromFiles("user",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "user.html"))
	r.AddFromFiles("alerts",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "alerts.html"),
		filepath.Join(templatesDir, "buttons.html"))
	r.AddFromFiles("classified",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "classified.html"),
		filepath.Join(templatesDir, "buttons.html"))
	r.AddFromFiles("single_host",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "single_host.html"),
		filepath.Join(templatesDir, "buttons.html"))
	r.AddFromFiles("single_host_data",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "single_host_data.html"),
		filepath.Join(templatesDir, "buttons.html"))
	r.AddFromFiles("export",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "export.html"))
	r.AddFromFiles("search",
		filepath.Join(templatesDir, "base.html"), filepath.Join(templatesDir, "search.html"),
		filepath.Join(templatesDir, "buttons.html"))

	return r
}
