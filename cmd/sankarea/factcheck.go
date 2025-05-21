package main

import (
    "fmt"
    "log"

    "github.com/bwmarrin/discordgo"
)

// FactCheckClaim runs a fact-check on the claim text via integrated APIs.
// Currently a stub - replace with real API calls.
func FactCheckClaim(claim string) string {
    // TODO: Integrate Google Fact Check API & ClaimBuster API here.
    // This is a placeholder response.
    return fmt.Sprintf("Fact-check for claim:\n\"%s\"\n\n[This is a stub. API integration coming soon.]", claim)
}

// Handle the /factcheck command interaction
func HandleFactCheckCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    claim := i.ApplicationCommandData().Options[0].StringValue()
    response := FactCheckClaim(claim)

    _, err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: response,
        },
    })

    if err != nil {
        log.Printf("Error responding to factcheck command: %v", err)
    }
}
