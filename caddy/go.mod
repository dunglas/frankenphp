module github.com/dunglas/frankenphp/caddy

go 1.20

replace github.com/dunglas/frankenphp => ../

replace (
	// some packages must match versions defined in go.mod of caddyserver/caddy/v2
	github.com/caddyserver/certmagic => github.com/caddyserver/certmagic v0.19.2
	github.com/google/cel-go => github.com/google/cel-go v0.15.1
	github.com/quic-go/quic-go => github.com/quic-go/quic-go v0.37.6
)

require (
	github.com/caddyserver/caddy/v2 v2.7.4
	github.com/dunglas/frankenphp v0.0.0-20230912200819-1f3007337d84
	github.com/dunglas/mercure/caddy v0.15.2
	github.com/dunglas/vulcain/caddy v0.5.0
	go.uber.org/automaxprocs v1.5.3
	go.uber.org/zap v1.25.0
)

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/Masterminds/sprig/v3 v3.2.3 // indirect
	github.com/MauriceGit/skiplist v0.0.0-20211105230623-77f5c8d3e145 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/RoaringBitmap/roaring v1.3.0 // indirect
	github.com/alecthomas/chroma/v2 v2.7.0 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/aryann/difflib v0.0.0-20210328193216-ff5ff6dc229b // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.8.0 // indirect
	github.com/caddyserver/certmagic v0.19.2 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/dgraph-io/badger v1.6.2 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dlclark/regexp2 v1.7.0 // indirect
	github.com/dunglas/httpsfv v1.0.1 // indirect
	github.com/dunglas/mercure v0.15.2 // indirect
	github.com/dunglas/vulcain v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/getkin/kin-openapi v0.118.0 // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-kit/kit v0.13.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-sql-driver/mysql v1.7.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/cel-go v0.18.0 // indirect
	github.com/google/certificate-transparency-go v1.1.4 // indirect
	github.com/google/go-tpm v0.3.3 // indirect
	github.com/google/go-tspi v0.3.0 // indirect
	github.com/google/pprof v0.0.0-20230912144702-c363fe2c2ed8 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/invopop/yaml v0.2.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.14.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.0 // indirect
	github.com/jackc/pgx/v4 v4.18.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kevburnsjr/skipfilter v0.0.1 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mastercactapus/proxyprotocol v0.0.4 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mholt/acmez v1.2.0 // indirect
	github.com/micromdm/scep/v2 v2.1.0 // indirect
	github.com/miekg/dns v1.1.56 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/onsi/ginkgo/v2 v2.12.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.3.4 // indirect
	github.com/quic-go/quic-go v0.38.1 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/slackhq/nebula v1.7.2 // indirect
	github.com/smallstep/certificates v0.24.3-rc1 // indirect
	github.com/smallstep/go-attestation v0.4.4-0.20230509120429-e17291421738 // indirect
	github.com/smallstep/nosql v0.6.0 // indirect
	github.com/smallstep/truststore v0.12.1 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.16.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	github.com/tailscale/tscert v0.0.0-20230806124524-28a91b69a046 // indirect
	github.com/tidwall/gjson v1.15.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/unrolled/secure v1.13.0 // indirect
	github.com/urfave/cli v1.22.14 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/goldmark v1.5.5 // indirect
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.42.0 // indirect
	go.opentelemetry.io/contrib/propagators/autoprop v0.42.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.17.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.17.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.17.0 // indirect
	go.opentelemetry.io/contrib/propagators/ot v1.17.0 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.16.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.16.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/sdk v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	go.step.sm/cli-utils v0.8.0 // indirect
	go.step.sm/crypto v0.35.0 // indirect
	go.step.sm/linkedca v0.20.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.13.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/term v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.13.0 // indirect
	google.golang.org/genproto v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230911183012-2d3300fd4832 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230911183012-2d3300fd4832 // indirect
	google.golang.org/grpc v1.58.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.0 // indirect
)
