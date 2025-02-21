module github.com/tdex-network/tdex-daemon

go 1.16

replace github.com/tdex-network/tdex-daemon/pkg/explorer => ./pkg/explorer

replace github.com/tdex-network/tdex-daemon/pkg/securestore => ./pkg/securestore

replace github.com/tdex-network/tdex-daemon/pkg/macaroons => ./pkg/macaroons

require (
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.1-0.20190118093823-f849b5445de4
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.10.0
	github.com/improbable-eng/grpc-web v0.13.0
	github.com/mdp/qrterminal/v3 v3.0.0
	github.com/prometheus/client_golang v1.0.0
	github.com/rs/cors v1.7.0 // indirect
	github.com/shopspring/decimal v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/soheilhy/cmux v0.1.5
	github.com/sony/gobreaker v0.4.1
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/tdex-network/tdex-daemon/pkg/explorer v0.0.0-20211001103242-a11e4485705a
	github.com/tdex-network/tdex-daemon/pkg/macaroons v0.0.0-20210813140257-70d50a8b72a4
	github.com/tdex-network/tdex-daemon/pkg/securestore v0.0.0-20210813140257-70d50a8b72a4
	github.com/tdex-network/tdex-protobuf v0.0.0-20220310134014-ac3f6ebd7d36
	github.com/thanhpk/randstr v1.0.4
	github.com/timshannon/badgerhold/v2 v2.0.0-20201016201833-94bc303c76d4
	github.com/urfave/cli/v2 v2.3.0
	github.com/vulpemventures/go-bip39 v1.0.2
	github.com/vulpemventures/go-elements v0.3.5
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	google.golang.org/genproto v0.0.0-20220317150908-0efb43f6373e
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/macaroon-bakery.v2 v2.3.0
	gopkg.in/macaroon.v2 v2.1.0
)
