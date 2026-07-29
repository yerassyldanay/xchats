module xchats-evals-harness

go 1.26.4

require (
	github.com/jackc/pgx/v5 v5.10.0
	github.com/pemistahl/lingua-go v1.4.0
	github.com/yerassyldanay/xchats/backend v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	golang.org/x/exp v0.0.0-20221106115401-f9659909a136 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/yerassyldanay/xchats/backend => ../../backend
