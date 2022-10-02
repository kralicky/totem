module github.com/kralicky/totem

go 1.18

require (
	github.com/charmbracelet/lipgloss v0.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.1.2
	github.com/jhump/protoreflect v1.13.0
	github.com/kralicky/gpkg v0.0.0-20220311205216-0d8ea9557555
	github.com/kralicky/ragu v1.0.0-rc1
	github.com/magefile/mage v1.13.0
	github.com/onsi/ginkgo/v2 v2.1.6
	github.com/onsi/gomega v1.20.2
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.34.0
	go.opentelemetry.io/contrib/propagators/autoprop v0.34.0
	go.opentelemetry.io/otel v1.10.0
	go.opentelemetry.io/otel/exporters/jaeger v1.9.0
	go.opentelemetry.io/otel/sdk v1.10.0
	go.opentelemetry.io/otel/trace v1.10.0
	go.uber.org/atomic v1.10.0
	go.uber.org/zap v1.23.0
	golang.org/x/exp v0.0.0-20220927162542-c76eaa363f9d
	golang.org/x/net v0.0.0-20220923203811-8be639271d50
	google.golang.org/genproto v0.0.0-20220923205249-dd2d53f1fffc
	google.golang.org/grpc v1.49.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/flosch/pongo2/v6 v6.0.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/iancoleman/strcase v0.2.0 // indirect
	github.com/kralicky/grpc-gateway/v2 v2.11.0-1 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/muesli/reflow v0.2.1-0.20210115123740-9e1d0d53df68 // indirect
	github.com/muesli/termenv v0.11.1-0.20220204035834-5ac8409525e0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/samber/lo v1.28.2 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.10.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.10.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.10.0 // indirect
	go.opentelemetry.io/contrib/propagators/ot v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/sys v0.0.0-20220728004956-3c1f35247d10 // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kralicky/ragu => ../ragu
