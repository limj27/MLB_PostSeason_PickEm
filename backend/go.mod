module pickem

go 1.22

require (
	github.com/go-sql-driver/mysql v1.8.1
	golang.org/x/crypto v0.26.0
)

require filippo.io/edwards25519 v1.1.0 // indirect

replace golang.org/x/crypto => github.com/golang/crypto v0.26.0

replace filippo.io/edwards25519 => github.com/FiloSottile/edwards25519 v1.1.0
