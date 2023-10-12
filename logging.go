package totem

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	gsync "github.com/kralicky/gpkg/sync"
	"github.com/lmittmann/tint"
	slogmulti "github.com/samber/slog-multi"
)

var (
	Log        *slog.Logger
	LogLevel   = slog.LevelWarn
	OutputPath = os.Stderr
)

var (
	colorCache  gsync.Map[string, lipgloss.Style]
	callerStyle = lipgloss.NewStyle().Faint(true)
)

const (
	ansiRed       = "\033[91m"
	ansiYellow    = "\033[93m"
	ansiBlue      = "\033[94m"
	ansiMagenta   = "\033[95m"
	ansiReset     = "\033[0m"
	ISO8601Format = "2006-01-02 15:04:05:000"
)

func parseLogLevel(levelStr string) slog.Level {
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = LogLevel
	}
	return level
}

func init() {
	colorCache.Store("totem", lipgloss.NewStyle().Background(lipgloss.Color("15")).Foreground(lipgloss.Color("0")))
	logLevel := LogLevel
	if levelStr, ok := os.LookupEnv("TOTEM_LOG_LEVEL"); ok {
		logLevel = parseLogLevel(levelStr)
	}

	options := &tint.Options{
		AddSource: true,
		Level:     logLevel,
		NoColor:   false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey: // format time
				a.Value = slog.StringValue(a.Value.Time().Format(ISO8601Format))
			case slog.SourceKey: // remove package name
				sb := strings.Builder{}
				switch cv := a.Value.Any().(type) {
				case *slog.Source:
					_, filename := filepath.Split(cv.File)
					sb.WriteString(filename)
					sb.WriteByte(':')
					sb.WriteString(strconv.Itoa(cv.Line))
					a.Value = slog.StringValue(callerStyle.Render(sb.String()))
				}
			case slog.LevelKey: // format level string
				switch cv := a.Value.Any().(type) {
				case slog.Level:
					lvlStr := strings.Builder{}
					switch cv {
					case slog.LevelDebug:
						lvlStr.WriteString(ansiMagenta)
						lvlStr.WriteString("DEBUG")
					case slog.LevelInfo:
						lvlStr.WriteString(ansiBlue)
						lvlStr.WriteString("INFO")
					case slog.LevelWarn:
						lvlStr.WriteString(ansiYellow)
						lvlStr.WriteString("WARN")
					case slog.LevelError:
						lvlStr.WriteString(ansiRed)
						lvlStr.WriteString("ERROR")
					default:
						return a
					}
					lvlStr.WriteString(ansiReset)
					a.Value = slog.StringValue(lvlStr.String())
				default:
					return a
				}
			}
			return a
		},
	}

	nameFormatter := newNameFormatMiddleware()

	Log = slog.New(slogmulti.Pipe(nameFormatter).Handler(tint.NewHandler(OutputPath, options))).WithGroup("totem")
}

func newRandomForegroundStyle() lipgloss.Style {
	r := math.Round(rand.Float64()*127) + 127
	g := math.Round(rand.Float64()*127) + 127
	b := math.Round(rand.Float64()*127) + 127
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", int(r), int(g), int(b))))
}

func newNameFormatMiddleware() slogmulti.Middleware {
	return func(next slog.Handler) slog.Handler {
		return &nameFormatMiddleware{
			next: next,
		}
	}
}

type nameFormatMiddleware struct {
	next   slog.Handler
	groups []string
}

func (h *nameFormatMiddleware) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *nameFormatMiddleware) Handle(ctx context.Context, record slog.Record) error {
	attrs := []slog.Attr{}

	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	sb := strings.Builder{}
	for i, part := range h.groups {
		if c, ok := colorCache.Load(part); !ok {
			newStyle := newRandomForegroundStyle()
			colorCache.Store(part, newStyle)
			sb.WriteString(newStyle.Render(part))
		} else {
			sb.WriteString(c.Render(part))
		}
		if i < len(h.groups)-1 {
			sb.WriteString(".")
		} else {
			sb.WriteByte(' ')
		}
	}

	// insert logger name before message
	sb.WriteString(record.Message)

	record = slog.NewRecord(record.Time, record.Level, sb.String(), record.PC)
	record.AddAttrs(attrs...)

	return h.next.Handle(ctx, record)
}

func (h *nameFormatMiddleware) WithAttrs(attrs []slog.Attr) slog.Handler {

	return &nameFormatMiddleware{
		next:   h.next.WithAttrs(attrs),
		groups: h.groups,
	}
}

func (h *nameFormatMiddleware) WithGroup(name string) slog.Handler {
	return &nameFormatMiddleware{
		next:   h.next.WithGroup(name),
		groups: append(h.groups, name),
	}
}
