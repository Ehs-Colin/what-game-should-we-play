package httpx

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/textutils"
)

const (
	internalErrorTitle   = "Something Went Wrong"
	internalErrorMessage = "The request failed. Try again."
)

func RespondWithError(responseWriter http.ResponseWriter, code int, message string) {
	message = textutils.StripANSI(message)
	if code >= 500 {
		fmt.Printf("Responding with error code %v, message: %v\n", code, message)
		message = internalErrorMessage
	}
	RespondWithJSON(responseWriter, code, map[string]string{"error": message})
}

func RespondWithJSON(responseWriter http.ResponseWriter, code int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(code)
	encoder := json.NewEncoder(responseWriter)
	if err := encoder.Encode(payload); err != nil {
		log.Printf("Error respondWithJSON: status code %v: %v", code, err)
	}
}
