package totem

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	gsync "github.com/kralicky/gpkg/sync"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log      *zap.Logger
	LogLevel = zap.NewAtomicLevelAt(zapcore.WarnLevel)
)

var (
	colorCache  gsync.Map[string, lipgloss.Style]
	callerStyle = lipgloss.NewStyle().Faint(true)
)

func init() {
	colorCache.Store("totem", lipgloss.NewStyle().Background(lipgloss.Color("15")).Foreground(lipgloss.Color("0")))
	if levelStr, ok := os.LookupEnv("TOTEM_LOG_LEVEL"); ok {
		if level, err := zapcore.ParseLevel(levelStr); err == nil {
			LogLevel.SetLevel(level)
		}
	}
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:    "M",
		LevelKey:      "L",
		TimeKey:       "T",
		NameKey:       "N",
		CallerKey:     "C",
		FunctionKey:   "",
		StacktraceKey: "S",
		LineEnding:    "\n",
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller: func(ec zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			path := ec.FullPath()
			// remove the package name
			if i := strings.LastIndex(path, "/"); i >= 0 {
				path = path[i+1:]
			}
			enc.AppendString(callerStyle.Render(path))
		},
		EncodeName: func(s string, enc zapcore.PrimitiveArrayEncoder) {
			parts := strings.Split(s, ".")
			sb := strings.Builder{}
			for i, part := range parts {
				if c, ok := colorCache.Load(part); !ok {
					newStyle := newRandomForegroundStyle()
					colorCache.Store(part, newStyle)
					sb.WriteString(newStyle.Render(part))
				} else {
					sb.WriteString(c.Render(part))
				}
				if i < len(parts)-1 {
					sb.WriteString(".")
				}
			}
			enc.AppendString(sb.String())
		},
		EncodeDuration:   zapcore.MillisDurationEncoder,
		ConsoleSeparator: " ",
		EncodeLevel:      zapcore.CapitalColorLevelEncoder,
	}
	zapConfig := zap.Config{
		Level:             LogLevel,
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Encoding:          "console",
		EncoderConfig:     encoderConfig,
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}
	lg, err := zapConfig.Build()
	if err != nil {
		panic(err)
	}
	Log = lg.Named("totem")
}

func newRandomForegroundStyle() lipgloss.Style {
	r := math.Round(rand.Float64()*127) + 127
	g := math.Round(rand.Float64()*127) + 127
	b := math.Round(rand.Float64()*127) + 127
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", int(r), int(g), int(b))))
}
