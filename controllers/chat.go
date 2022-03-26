package controllers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"context"

	"github.com/owncast/owncast/core/chat"
	"github.com/owncast/owncast/core/user"
	"github.com/owncast/owncast/router/middleware"
	"github.com/ravener/discord-oauth2"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"golang.org/x/oauth2"
)

func conf() (conf oauth2.Config) {
	conf = oauth2.Config{
		RedirectURL: "http://localhost:8080/authorized/",
		// WIP, set and grab id and secret from config.
		ClientID:     "id",
		ClientSecret: "secret",
		Scopes:       []string{discord.ScopeIdentify},
		Endpoint:     discord.Endpoint,
	}
	return
}

func RedirectToDiscordOauth(w http.ResponseWriter, r *http.Request) {
	conf := conf()

	// WIP Need an actual state for this
	http.Redirect(w, r, conf.AuthCodeURL("test"), http.StatusTemporaryRedirect)
}

func HandleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	conf := conf()

	token, err := conf.Exchange(context.Background(), r.FormValue("code"))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	// get user info
	res, err := conf.Client(context.Background(), token).Get("https://discord.com/api/users/@me")

	if err != nil || res.StatusCode != 200 {
		w.WriteHeader(http.StatusInternalServerError)
		if err != nil {
			w.Write([]byte(err.Error()))
		} else {
			w.Write([]byte(res.Status))
		}
		return
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)

	username := gjson.Get(
		string(body),
		"username",
	).Str

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	response := RegisterDiscordUser(w, username, token.AccessToken)
	http.Redirect(w, r, "/?user_id="+response.ID, http.StatusTemporaryRedirect)
	return
}

// ExternalGetChatMessages gets all of the chat messages.
func ExternalGetChatMessages(integration user.ExternalAPIUser, w http.ResponseWriter, r *http.Request) {
	middleware.EnableCors(w)
	GetChatMessages(w, r)
}

// GetChatMessages gets all of the chat messages.
func GetChatMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		messages := chat.GetChatHistory()

		if err := json.NewEncoder(w).Encode(messages); err != nil {
			log.Debugln(err)
		}
	default:
		w.WriteHeader(http.StatusNotImplemented)
		if err := json.NewEncoder(w).Encode(j{"error": "method not implemented (PRs are accepted)"}); err != nil {
			InternalErrorHandler(w, err)
		}
	}
}

// RegisterAnonymousChatUser will register a new user.
func RegisterAnonymousChatUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != POST {
		WriteSimpleResponse(w, false, r.Method+" not supported")
		return
	}

	type registerAnonymousUserRequest struct {
		DisplayName string `json:"displayName"`
	}

	type registerAnonymousUserResponse struct {
		ID          string `json:"id"`
		AccessToken string `json:"accessToken"`
		DisplayName string `json:"displayName"`
	}

	decoder := json.NewDecoder(r.Body)
	var request registerAnonymousUserRequest
	if err := decoder.Decode(&request); err != nil { //nolint
		// this is fine. register a new user anyway.
	}

	if request.DisplayName == "" {
		request.DisplayName = r.Header.Get("X-Forwarded-User")
	}

	newUser, err := user.CreateAnonymousUser(request.DisplayName)
	if err != nil {
		WriteSimpleResponse(w, false, err.Error())
		return
	}

	response := registerAnonymousUserResponse{
		ID:          newUser.ID,
		AccessToken: newUser.AccessToken,
		DisplayName: newUser.DisplayName,
	}

	w.Header().Set("Content-Type", "application/json")
	middleware.DisableCache(w)

	WriteResponse(w, response)
}

type registerDiscordUserResponse struct {
	ID          string `json:"id"`
	AccessToken string `json:"accessToken"`
	DisplayName string `json:"displayName"`
}

func RegisterDiscordUser(w http.ResponseWriter, username string, token string) (response registerDiscordUserResponse) {
	// Register discord user as chat user

	type registerDiscordUserRequest struct {
		DisplayName string `json:"displayName"`
	}

	discordUser, err := user.CreateDiscordUser(username, token)
	if err != nil {
		WriteSimpleResponse(w, false, err.Error())
		return
	}
	return registerDiscordUserResponse{
		ID:          discordUser.ID,
		AccessToken: discordUser.AccessToken,
		DisplayName: discordUser.DisplayName,
	}
}
