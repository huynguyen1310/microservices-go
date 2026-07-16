package main

import (
	"encoding/json"
	"net/http"
	"ride-sharing/shared/contracts"
)

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	defer r.Body.Close()

	if reqBody.UserID != "" {
		writeJSON(w, http.StatusBadRequest, "userID is required")
		return
	}

	response := contracts.APIResponse{
		Data: "ok",
	}

	writeJSON(w, http.StatusCreated, response)
}
