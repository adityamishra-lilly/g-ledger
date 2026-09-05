package config

import "flag"

var (
    Host string
    Port int
)

func Parse() {
    flag.StringVar(&Host, "host", "localhost", "server host")
    flag.IntVar(&Port, "port", 8080, "server port")
    flag.Parse()
}
