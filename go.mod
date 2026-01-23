module github.com/apito-io/engine

go 1.25.0

require (
	github.com/99designs/gqlgen v0.17.76
	github.com/JohannesKaufmann/html-to-markdown v1.6.0
	github.com/RashadAnsari/go-batch/v2 v2.1.0
	github.com/ThreeDotsLabs/watermill v1.5.0
	github.com/ThreeDotsLabs/watermill-bolt v1.2.1
	github.com/ThreeDotsLabs/watermill-redisstream v1.4.4
	github.com/apito-io/apitoBolt v0.1.1
	github.com/apito-io/oas v1.2.3
	github.com/apito-io/types v0.1.4
	github.com/apito-io/wsgraphql v1.5.4
	github.com/aws/aws-sdk-go-v2 v1.36.5
	github.com/aws/aws-sdk-go-v2/config v1.29.17
	github.com/aws/aws-sdk-go-v2/credentials v1.17.70
	github.com/aws/aws-sdk-go-v2/service/ses v1.30.5
	github.com/cloudflare/tableflip v1.2.3
	github.com/coreos/go-oidc v2.3.0+incompatible
	github.com/edwingeng/hotswap v1.0.0
	github.com/elliotchance/pie v1.39.0
	github.com/fatih/camelcase v1.0.0
	github.com/getsentry/sentry-go v0.34.1
	github.com/gin-gonic/gin v1.10.1
	github.com/go-sql-driver/mysql v1.9.3
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/graph-gophers/dataloader v5.0.0+incompatible
	github.com/graph-gophers/dataloader/v7 v7.1.0
	github.com/h2non/go-is-svg v0.0.0-20160927212452-35e8c4b0612c
	github.com/hako/branca v0.0.0-20200807062402-6052ac720505
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-plugin v1.6.3
	github.com/iancoleman/strcase v0.3.0
	github.com/ilyakaznacheev/cleanenv v1.5.0
	github.com/jboursiquot/go-proverbs v0.0.2
	github.com/jinzhu/inflection v1.0.0
	github.com/k3a/html2text v1.2.1
	github.com/labstack/echo-contrib v0.17.4
	github.com/labstack/echo/v4 v4.13.4
	github.com/liangyaopei/structmap v0.0.0-20200911074950-0e2911025a96
	github.com/machinebox/graphql v0.2.2
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/nitishm/go-rejson/v4 v4.2.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/puzpuzpuz/xsync/v3 v3.5.1
	github.com/rabbitmq/amqp091-go v1.10.0
	github.com/redis/go-redis/v9 v9.12.1
	github.com/sirupsen/logrus v1.9.3
	github.com/tailor-platform/graphql v0.6.0
	github.com/teivah/onecontext v1.3.0
	github.com/tomogoma/go-api-guard v0.0.0-20180312041446-a2bad766ec64
	github.com/tomogoma/go-typed-errors v0.0.0-20181222204503-0532faf740be
	github.com/uptrace/bun v1.2.15
	github.com/uptrace/bun/dialect/mysqldialect v1.2.15
	github.com/uptrace/bun/dialect/pgdialect v1.2.15
	github.com/uptrace/bun/dialect/sqlitedialect v1.2.15
	github.com/uptrace/bun/driver/pgdriver v1.2.15
	github.com/uptrace/bun/driver/sqliteshim v1.2.15
	github.com/vektah/gqlparser/v2 v2.5.30
	github.com/yuin/goldmark v1.7.13
	go.etcd.io/bbolt v1.4.2
	go.mongodb.org/mongo-driver v1.17.4
	go.mongodb.org/mongo-driver/v2 v2.2.2
	go.opentelemetry.io/otel v1.37.0
	golang.org/x/crypto v0.40.0
	golang.org/x/net v0.42.0
	golang.org/x/oauth2 v0.30.0
	golang.org/x/sync v0.16.0
	google.golang.org/api v0.242.0
	google.golang.org/grpc v1.73.0
	google.golang.org/protobuf v1.36.8
	gopkg.in/h2non/filetype.v1 v1.0.5
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go/auth v0.16.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/PuerkitoBio/goquery v1.10.3 // indirect
	github.com/Rican7/retry v0.3.1 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.34.0 // indirect
	github.com/aws/smithy-go v1.22.4 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/boreq/errors v0.1.0 // indirect
	github.com/bytedance/sonic v1.11.6 // indirect
	github.com/bytedance/sonic/loader v0.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/edwingeng/slog v0.0.0-20221027170832-482f0dfb6247 // indirect
	github.com/eknkc/basex v1.0.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.2 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.7 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lithammer/shortuuid/v3 v3.0.7 // indirect
	github.com/matryer/is v1.4.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.28 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/oklog/run v1.2.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/pierrec/xxHash v0.1.5 // indirect
	github.com/pquerna/cachecontrol v0.2.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/tomogoma/generator v0.0.0-20171014125632-4398aab4dd41 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/exp v0.0.0-20250711185948-6ae5c78190dc // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250715232539-7130f93afb79 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.3 // indirect
	mellium.im/sasl v0.3.2 // indirect
	modernc.org/libc v1.66.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.38.0 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)
