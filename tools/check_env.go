package main

import (
    "fmt"
    "os"
)

func main() {
    required := []string{
        "DISCORD_BOT_TOKEN",
        "DISCORD_APPLICATION_ID",
        "DISCORD_GUILD_ID",
        "DISCORD_CHANNEL_ID",
    }
    missing := []string{}

    for _, key := range required {
        val := os.Getenv(key)
        if val == "" {
            missing = append(missing, key)
        } else {
            fmt.Printf("%s is set\n", key)
        }
    }
    if len(missing) > 0 {
        fmt.Printf("Missing environment variables: %v\n", missing)
        os.Exit(1)
    } else {
        fmt.Println("All required environment variables are set.")
    }
}
