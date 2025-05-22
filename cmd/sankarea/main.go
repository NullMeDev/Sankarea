package main

import (
    "fmt"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    defer logPanic()
    fmt.Println("Sankarea bot starting up...")

    LoadEnv()
    if err := SetupLogging(); err != nil {
        log.Printf("Warning: %v", err)
    }

    FileMustExist("config/config.json")
    FileMustExist("config/sources.yml")
    EnsureDataDir()

    // TODO: Load config, sources, state and initialize Discord session
    // TODO: Register commands and handlers
    // TODO: Setup cron jobs and backups
    // TODO: Open session and wait for signals

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    Logger().Println("Sankarea bot shutting down...")
}
