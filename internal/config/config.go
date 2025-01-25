package config

import "os"

type Config struct {
	Port             string
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
	RedisHost        string
	RedisPort        string
	RedisPassword    string
	LogFile          string
}

func Load() *Config {
	return &Config{
		Port:             os.Getenv("APP_PORT"),
		DatabaseHost:     os.Getenv("POSTGRES_HOST"),
		DatabasePort:     os.Getenv("POSTGRES_PORT"),
		DatabaseUser:     os.Getenv("POSTGRES_USER"),
		DatabasePassword: os.Getenv("POSTGRES_PASSWORD"),
		DatabaseName:     os.Getenv("POSTGRES_DB"),
		RedisHost:        os.Getenv("REDIS_HOST"),
		RedisPort:        os.Getenv("REDIS_PORT"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		LogFile:          os.Getenv("LOG_FILE"),
	}
}
