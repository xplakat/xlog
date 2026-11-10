// Package xlog is simple logging module for go, with a rotating file feature
// and console logging.
package xlog

import (
	"github.com/xplakat/xcolor"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync/atomic"
	"time"
)

// LogLevel type
type LogLevel int32

const (
	// LevelTrace logs everything
	LevelTrace LogLevel = (1 << iota)

	// LevelInfo logs Info, Warnings and Errors
	LevelInfo

	// LevelWarn logs Warning and Errors
	LevelWarn

	// LevelError logs just Errors
	LevelError
)

const MaxBytes int = 10 * 1024 * 1024
const BackupCount int = 10

type xlog struct {
	LogLevel int32

	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
	Fatal   *log.Logger

	LogFile     *RotatingFileHandler
	GlobalWarn  bool
	GlobalError bool
}

var Logger xlog

// DefaultFlags used by created loggers
var DefaultFlags = log.Ldate | log.Ltime | log.Lshortfile

// RotatingFileHandler writes log a file, if file size exceeds maxBytes,
// it will backup current file and open a new one.
//
// max backup file number is set by backupCount, it will delete oldest if backups too many.
type RotatingFileHandler struct {
	fd *os.File

	fileName    string
	maxBytes    int
	backupCount int
}

// NewRotatingFileHandler => creates dirs and opens the logfile
func NewRotatingFileHandler(fileName string, maxBytes int, backupCount int) (*RotatingFileHandler, error) {
	dir := path.Dir(fileName)
	os.Mkdir(dir, 0777)

	h := new(RotatingFileHandler)

	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid max bytes")
	}

	h.fileName = fileName
	h.maxBytes = maxBytes
	h.backupCount = backupCount

	var err error
	h.fd, err = os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// Write => the rotating file
func (h *RotatingFileHandler) Write(p []byte) (n int, err error) {
	h.doRollover()
	return h.fd.Write(p)
}

// Close => simply closes the log File
func (h *RotatingFileHandler) Close() error {
	if h.fd != nil {
		return h.fd.Close()
	}
	return nil
}

// doRollover => File roll over
func (h *RotatingFileHandler) doRollover() {
	f, err := h.fd.Stat()
	if err != nil {
		return
	}

	// log.Println("size: ", f.Size())

	if h.maxBytes <= 0 {
		return
	} else if f.Size() < int64(h.maxBytes) {
		return
	}

	if h.backupCount > 0 {
		h.fd.Close()

		for i := h.backupCount - 1; i > 0; i-- {
			sfn := fmt.Sprintf("%s.%d", h.fileName, i)
			dfn := fmt.Sprintf("%s.%d", h.fileName, i+1)

			os.Rename(sfn, dfn)
		}

		dfn := fmt.Sprintf("%s.1", h.fileName)
		os.Rename(h.fileName, dfn)

		h.fd, _ = os.OpenFile(h.fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}
}

// Start => starts the logging
func Start(level LogLevel, path string) {
	doLogging(level, path, MaxBytes, BackupCount)
}

// StartEx => starts the loggins with a specific number for [maxBytes] and [backupCount]
func StartEx(level LogLevel, path string, maxBytes, backupCount int) {
	doLogging(level, path, maxBytes, backupCount)
}

// Stop => stops the logging
func Stop() error {
	if Logger.LogFile != nil {
		return Logger.LogFile.Close()
	}

	return nil
}

// Sync => commits the current contents of the file to stable storage.
// Typically, this means flushing the file system's in-memory copy
// of recently written data to disk.
func Sync() {
	if Logger.LogFile != nil {
		Logger.LogFile.fd.Sync()
	}
}

func doLogging(logLevel LogLevel, fileName string, maxBytes, backupCount int) {
	traceHandle := io.Discard
	infoHandle := io.Discard
	warnHandle := io.Discard
	errorHandle := io.Discard
	fatalHandle := io.Discard

	var fileHandle *RotatingFileHandler

	switch logLevel {
	case LevelTrace:
		traceHandle = os.Stdout
		fallthrough
	case LevelInfo:
		infoHandle = os.Stdout
		fallthrough
	case LevelWarn:
		warnHandle = os.Stdout
		fallthrough
	case LevelError:
		errorHandle = os.Stderr
		fatalHandle = os.Stderr
	}

	if fileName != "" {
		var err error
		fileHandle, err = NewRotatingFileHandler(fileName, maxBytes, backupCount)
		if err != nil {
			log.Fatal("xlog: unable to create RotatingFileHandler: ", err)
		}

		if traceHandle == os.Stdout {
			traceHandle = io.MultiWriter(fileHandle, traceHandle)
		}

		if infoHandle == os.Stdout {
			infoHandle = io.MultiWriter(fileHandle, infoHandle)
		}

		if warnHandle == os.Stdout {
			warnHandle = io.MultiWriter(fileHandle, warnHandle)
		}

		if errorHandle == os.Stderr {
			errorHandle = io.MultiWriter(fileHandle, errorHandle)
		}

		if fatalHandle == os.Stderr {
			fatalHandle = io.MultiWriter(fileHandle, fatalHandle)
		}
	}

	Logger = xlog{
		Trace:       log.New(traceHandle, "", DefaultFlags),
		Info:        log.New(infoHandle, "", DefaultFlags),
		Warning:     log.New(warnHandle, "", DefaultFlags),
		Error:       log.New(errorHandle, "", DefaultFlags),
		Fatal:       log.New(fatalHandle, "", DefaultFlags),
		LogFile:     fileHandle,
		GlobalWarn:  false,
		GlobalError: false,
	}

	atomic.StoreInt32(&Logger.LogLevel, int32(logLevel))
}

//** TRACE

// Trace => writes to the Trace destination
func Trace(format string, a ...interface{}) {
	if Logger.LogLevel == 1 {
		fmt.Print(xcolor.CyanString("■■ "))
	}
	Logger.Trace.Output(2, fmt.Sprintf("[TRACE] "+format, a...))
}

//** INFO

// Info => writes to the Info destination
func Info(format string, a ...interface{}) {
	if Logger.LogLevel <= 2 {
		fmt.Print(xcolor.GreenString("■■ "))
	}
	Logger.Info.Output(2, fmt.Sprintf("[INFO]  "+format, a...))
}

//** WARNING

// Warning => writes to the Warning destination
func Warning(format string, a ...interface{}) {
	if Logger.LogLevel <= 4 {
		Logger.GlobalWarn = true
		fmt.Print(xcolor.YellowString("■■ "))
	}
	Logger.Warning.Output(2, fmt.Sprintf("[WARN]  "+format, a...))
}

//** ERROR

// Error => writes to the Error destination and accepts an err
func Error(err error) {
	if Logger.LogLevel <= 8 {
		Logger.GlobalError = true
		fmt.Print(xcolor.RedString("■■ "))
	}
	Logger.Error.Output(2, fmt.Sprintf("[ERROR] %s\n", err))
}

// IfError => is a shortcut function for log.Error if error
func IfError(err error) {
	if err != nil {
		if Logger.LogLevel <= 8 {
			Logger.GlobalError = true
			fmt.Print(xcolor.RedString("■■ "))
		}
		Logger.Error.Output(2, fmt.Sprintf("[ERROR] %s\n", err))
	}
}

//** FATAL

// Fatal => writes to the Fatal destination and exits with an error 255 code
func Fatal(format string, a ...interface{}) {
	fmt.Print(xcolor.MagentaString("■■ "))
	Logger.Fatal.Output(2, fmt.Sprintf("[FATAL] "+format, a...))
	Sync()
	os.Exit(255)
}

// Fatalf => writes to the Fatal destination and exits with an error 255 code
func Fatalf(format string, a ...interface{}) {
	fmt.Print(xcolor.MagentaString("■■ "))
	Logger.Fatal.Output(2, fmt.Sprintf("[FATAL] "+format, a...))
	Sync()
	os.Exit(255)
}

// FatalIfError => is a shortcut function for log.Fatalf if error and
// exits with an error 255 code
func FatalIfError(err error) {
	if err != nil {
		fmt.Print(xcolor.MagentaString("■■ "))
		Logger.Fatal.Output(2, fmt.Sprintf("[FATAL] %s\n", err))
		Sync()
		os.Exit(255)
	}
}

// FatalNotExit => writes to the Fatal destination but not exit
func FatalNotExit(err error) {
	Logger.GlobalError = true
	fmt.Print(xcolor.MagentaString("■■ "))
	Logger.Fatal.Output(2, fmt.Sprintf("[FATAL] %s\n", err))
}

// ProgressBar => Progress bar on the terminal
func ProgressBar(position int) {
	dateNow := time.Now().Format("2006/02/01 15:04:05")
	pBar := ""
	i := 0
	if position >= 0 && position <= 100 {
		stepBar := position / 10
		for i < 10 {
			if i < stepBar {
				pBar += "■■■"
			} else {
				pBar += "   "
			}
			i++
		}
		fmt.Printf("\r"+xcolor.BlackLgtString("■■ ")+dateNow+" [TRMT]  ["+pBar+"] %d%%", position)
	}
}

// Init => Init the log format
func Init(logLevel, logFile string) {
	switch logLevel {
	case "TRACE":
		DefaultFlags = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile
		Start(LevelTrace, logFile)
	case "INFO":
		DefaultFlags = log.Ldate | log.Ltime
		Start(LevelInfo, logFile)
	case "WARN":
		DefaultFlags = log.Ldate | log.Ltime
		Start(LevelWarn, logFile)
	case "ERROR":
		DefaultFlags = log.Ldate | log.Ltime
		Start(LevelError, logFile)
	default:
		DefaultFlags = log.Ldate | log.Ltime
		Start(LevelInfo, logFile)
	}
}
