module xchats-evals-harness

go 1.26.4

require (
	github.com/pemistahl/lingua-go v1.4.0
	github.com/yerassyldanay/xchats/backend v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/shopspring/decimal v1.3.1 // indirect
	golang.org/x/exp v0.0.0-20221106115401-f9659909a136 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/yerassyldanay/xchats/backend => ../../backend
