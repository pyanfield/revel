package revel

import (
	"github.com/robfig/config"
	"go/build"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	REVEL_IMPORT_PATH = "github.com/robfig/revel"
)

var (
	// App details
	AppName    string // e.g. "sample"
	BasePath   string // e.g. "/Users/robfig/gocode/src/corp/sample"
	AppPath    string // e.g. "/Users/robfig/gocode/src/corp/sample/app"
	ViewsPath  string // e.g. "/Users/robfig/gocode/src/corp/sample/app/views"
	ImportPath string // e.g. "corp/sample"
	SourcePath string // e.g. "/Users/robfig/gocode/src"

	Config  *MergedConfig
	RunMode string // Application-defined (by default, "dev" or "prod")

	// Revel installation details
	RevelPath string // e.g. "/Users/robfig/gocode/src/revel"

	// Where to look for templates and configuration.
	// Ordered by priority.  (Earlier paths take precedence over later paths.)
	// 定义了 template 和 configuration 文件的位置
	// 按照优先级排序 (早先的 paths 优先于 后来的 paths)
	CodePaths     []string
	ConfPaths     []string
	TemplatePaths []string

	Modules []Module

	// Server config.
	//
	// Alert: This is how the app is configured, which may be different from
	// the current process reality.  For example, if the app is configured for
	// port 9000, HttpPort will always be 9000, even though in dev mode it is
	// run on a random port and proxied.
	HttpPort int    // e.g. 9000
	HttpAddr string // e.g. "", "127.0.0.1"

	// All cookies dropped by the framework begin with this prefix.
	CookiePrefix string

	// Loggers
	// 定义了默认的 logger 信息的格式， 以""为开头，然后是日期，时间，和文件名及行数
	// 格式可以参见使用命令行启动你的app，如： revel run hello_revel
	DEFAULT = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	TRACE   = DEFAULT
	INFO    = DEFAULT
	WARN    = DEFAULT
	ERROR   = DEFAULT

	Initialized bool

	// Private
	secretKey []byte
)

func init() {
	log.SetFlags(DEFAULT.Flags())
}

// Init initializes Revel -- it provides paths for getting around the app.
//
// Params:
//   mode - the run mode, which determines which app.conf settings are used.
//   importPath - the Go import path of the application.
//   srcPath - the path to the source directory, containing Revel and the app.
//     If not specified (""), then a functioning Go installation is required.
func Init(mode, importPath, srcPath string) {
	// Ignore trailing slashes.
	ImportPath = strings.TrimRight(importPath, "/")
	SourcePath = srcPath
	RunMode = mode

	// If the SourcePath is not specified, find it using build.Import.
	var revelSourcePath string // may be different from the app source path
	if SourcePath == "" {
		//根据 importPath 得到revelSourcePath 和 SourcePath 的 root directory
		revelSourcePath, SourcePath = findSrcPaths(importPath)
	} else {
		// If the SourcePath was specified, assume both Revel and the app are within it.
		SourcePath = path.Clean(SourcePath)
		revelSourcePath = SourcePath
	}

	RevelPath = path.Join(revelSourcePath, filepath.FromSlash(REVEL_IMPORT_PATH))
	BasePath = path.Join(SourcePath, filepath.FromSlash(importPath))
	AppPath = path.Join(BasePath, "app")
	ViewsPath = path.Join(AppPath, "views")

	CodePaths = []string{AppPath}

	ConfPaths = []string{
		path.Join(BasePath, "conf"),
		path.Join(RevelPath, "conf"),
	}

	TemplatePaths = []string{
		ViewsPath,
		path.Join(RevelPath, "templates"),
	}

	// Load app.conf
	var err error
	Config, err = LoadConfig("app.conf")
	if err != nil || Config == nil {
		log.Fatalln("Failed to load app.conf:", err)
	}
	// Ensure that the selected runmode appears in app.conf.
	// If empty string is passed as the mode, treat it as "DEFAULT"
	if mode == "" {
		mode = config.DEFAULT_SECTION
	}
	if !Config.HasSection(mode) {
		log.Fatalln("app.conf: No mode found:", mode)
	}
	Config.SetSection(mode)

	// Configure properties from app.conf
	HttpPort = Config.IntDefault("http.port", 9000)
	HttpAddr = Config.StringDefault("http.addr", "")
	AppName = Config.StringDefault("app.name", "(not set)")
	CookiePrefix = Config.StringDefault("cookie.prefix", "REVEL")
	secretStr := Config.StringDefault("app.secret", "")
	if secretStr == "" {
		log.Fatalln("No app.secret provided.")
	}
	secretKey = []byte(secretStr)

	// Configure logging.
	TRACE = getLogger("trace")
	INFO = getLogger("info")
	WARN = getLogger("warn")
	ERROR = getLogger("error")

	loadModules()

	Initialized = true
}

// Create a logger using log.* directives in app.conf plus the current settings
// on the default logger.
func getLogger(name string) *log.Logger {
	var logger *log.Logger

	// Create a logger with the requested output. (default to stderr)
	output := Config.StringDefault("log."+name+".output", "stderr")

	switch output {
	case "stdout":
		logger = newLogger(os.Stdout)
	case "stderr":
		logger = newLogger(os.Stderr)
	default:
		if output == "off" {
			output = os.DevNull
		}

		file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalln("Failed to open log file", output, ":", err)
		}
		logger = newLogger(file)
	}

	// Set the prefix / flags.
	flags, found := Config.Int("log." + name + ".flags")
	if found {
		logger.SetFlags(flags)
	}

	prefix, found := Config.String("log." + name + ".prefix")
	if found {
		logger.SetPrefix(prefix)
	}

	return logger
}

func newLogger(wr io.Writer) *log.Logger {
	return log.New(wr, DEFAULT.Prefix(), DEFAULT.Flags())
}

// findSrcPaths uses the "go/build" package to find the source root for Revel
// and the app.
func findSrcPaths(importPath string) (revelSourcePath, appSourcePath string) {
	// 通过 os.Getenv() 去查找环境变量的值，如果查找不到，返回空
	// 如果 GOPATH 没有定义，提示错误信息，要求用户定义 go 环境的 GOPATH
	if gopath := os.Getenv("GOPATH"); gopath == "" {
		ERROR.Fatalln("GOPATH environment variable is not set. ",
			"Please refer to http://golang.org/doc/code.html to configure your Go environment.")
	}
	// 通过 FindOnly 的形式导入路径，即，在定位了某个包所在文件夹的位置，Import停止，不会读取任何此文件夹中的文件
	appPkg, err := build.Import(importPath, "", build.FindOnly)
	if err != nil {
		ERROR.Fatalln("Failed to import", importPath, "with error:", err)
	}

	revelPkg, err := build.Import(REVEL_IMPORT_PATH, "", build.FindOnly)
	if err != nil {
		ERROR.Fatalln("Failed to find Revel with error:", err)
	}
	// return package source root directory
	return revelPkg.SrcRoot, appPkg.SrcRoot
}

type Module struct {
	Name, ImportPath, Path string
}

func loadModules() {
	for _, key := range Config.Options("module.") {
		moduleImportPath := Config.StringDefault(key, "")
		if moduleImportPath == "" {
			continue
		}

		modPkg, err := build.Import(moduleImportPath, "", build.FindOnly)
		if err != nil {
			log.Fatalln("Failed to load module.  Import of", moduleImportPath, "failed:", err)
		}

		addModule(key[len("module."):], moduleImportPath, modPkg.Dir)
	}
}

func addModule(name, importPath, modulePath string) {
	Modules = append(Modules, Module{Name: name, ImportPath: importPath, Path: modulePath})
	if codePath := path.Join(modulePath, "app"); DirExists(codePath) {
		CodePaths = append(CodePaths, codePath)
	}
	if viewsPath := path.Join(modulePath, "app", "views"); DirExists(viewsPath) {
		TemplatePaths = append(TemplatePaths, viewsPath)
	}
	INFO.Print("Loaded module ", path.Base(modulePath))

	// Hack: There is presently no way for the testrunner module to add the
	// "test" subdirectory to the CodePaths.  So this does it instead.
	if importPath == "github.com/robfig/revel/modules/testrunner" {
		CodePaths = append(CodePaths, path.Join(BasePath, "tests"))
	}
}

func CheckInit() {
	if !Initialized {
		panic("Revel has not been initialized!")
	}
}
