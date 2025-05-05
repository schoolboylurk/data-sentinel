package handlers

import (
	"fmt"

	"github.com/schoolboylurk/data-sentinel/pkg/database"
)

// WrapPromptWithPolicy loads the kid's age and content policy, then constructs the system message
// and returns the full prompt for the AI.
func WrapPromptWithPolicy(kid, prompt string) string {
	var age int
	var allowList, restrictList string
	// Retrieve age
	if err := database.DB.QueryRow(
		"SELECT age FROM kids WHERE username = ?", kid,
	).Scan(&age); err != nil {
		// Default age or log error if needed
		age = 0
	}

	// Retrieve allow and restrict lists
	if err := database.DB.QueryRow(
		"SELECT allowed, restricted FROM content_policies WHERE kid_username = ?", kid,
	).Scan(&allowList, &restrictList); err != nil {
		// Default to empty lists or log error
		allowList, restrictList = "", ""
	}

	sysMsg := fmt.Sprintf(
		"You are an AI assistant for a %d-year-old. Allowed topics: %s. Restricted topics: %s.",
		age, allowList, restrictList,
	)
	return sysMsg + "\nUser asks: " + prompt
}
