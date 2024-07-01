package authentication

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"

	"SalesforceGit/db"
)

const grantType string = "urn:ietf:params:oauth:grant-type:jwt-bearer"


// AuthenticationRequest - wrapper to hold auth request data
// OAuth 2.0. protocol is used
type AuthRequest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	// ClientID string `json:"client_iD"`
}

type authenticationResponse struct {
	Token       string `json:"access_token"`
	InstanceURL string `json:"instance_url"`
	ID          string `json:"id"`
	TokenType   string `json:"token_type"`
	IssuedAt    string `json:"issued_at"`
	Signature   string `json:"signature"`
}

type claims struct {
	jwt.StandardClaims
}

type AuthenticationResponse interface {
	GetToken() string
	GetInstanceURL() string
	GetID() string
	GetTokenType() string
	GetIssuedAt() string
	GetSignature() string
}

// GetToken returns the authenication token.
func (response authenticationResponse) GetToken() string { return response.Token }

// GetInstanceURL returns the Salesforce instance URL to use with the authenication information.
func (response authenticationResponse) GetInstanceURL() string { return response.InstanceURL }

// GetID returns the Salesforce ID of the authenication.
func (response authenticationResponse) GetID() string { return response.ID }

// GetTokenType returns the authenication token type.
func (response authenticationResponse) GetTokenType() string { return response.TokenType }

// GetIssuedAt returns the time when the token was issued.
func (response authenticationResponse) GetIssuedAt() string { return response.IssuedAt }

// GetSignature returns the signature of the authenication.
func (response authenticationResponse) GetSignature() string { return response.Signature }

// Authenicate will exchange the JWT signed request for access token.
func Authenticate(request AuthRequest, client *http.Client) (AuthenticationResponse, error) {

	privateKeyFile, err := os.Open("privatekey.pem") // Give a relative reference to this file.

	if err != nil {
		return nil, err
	}

	pemfileinfo, _ := privateKeyFile.Stat()

	var size int64 = pemfileinfo.Size()

	pembytes := make([]byte, size)

	buffer := bufio.NewReader(privateKeyFile)
	_, err = buffer.Read(pembytes)

	if err != nil {
		return nil, err
	}
	pemData := []byte(pembytes)

	privateKeyFile.Close() // close file

	expirationTime := time.Now().Add(1 * time.Minute)
	integrationUserValues, err := db.GetSubscribeValues()
	consumerKey := integrationUserValues.SalesforceKey
	claims := &claims{
		StandardClaims: jwt.StandardClaims{
			// In JWT, the expiry time is expressed as unix milliseconds
			ExpiresAt: expirationTime.Unix(), // type int64
			Audience:  request.URL,           // "https://test.salesforce.com" || "https://login.salesforce.com"
			Issuer:    consumerKey,           // consumer key of the connected app, hardcoded
			Subject:   request.Username,      // username of the salesforce user, whose profile is added to the connected app
		},
	}
	// fmt.Println(claims.ExpiresAt)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	token.Header = map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	signKey, err := jwt.ParseRSAPrivateKeyFromPEM(pemData) // parse the RSA key

	if err != nil {
		return nil, err
	}

	tokenString, err := token.SignedString(signKey) // sign the claims with private key

	if err != nil {
		return nil, err
	}
	// fmt.Println(tokenString)
	form := url.Values{}
	form.Add("grant_type", grantType)
	form.Add("assertion", tokenString)

	urlForEndpoint := request.URL + "/services/oauth2/token" // token endpoint for getting access token
	// log.Info("Performing request using url: " + url)
	// fmt.Println(urlForEndpoint)
	// fmt.Println(strings.NewReader(form.Encode()))
	httpRequest, err := http.NewRequest("POST", urlForEndpoint, strings.NewReader(form.Encode()))
	// fmt.Println(httpRequest)

	if err != nil {
		return nil, err
	}

	httpRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, respErr := client.Do(httpRequest)
	// fmt.Println(response)
	if respErr != nil {
		return nil, respErr
	}

	if response.StatusCode >= 300 {
		defer response.Body.Close() // Close the response body to prevent resource leaks
		responseBody, _ := io.ReadAll(response.Body)
		fmt.Println("Response Body:", string(responseBody)) // Print the response body
		return nil, errors.New(response.Status)
	}

	body, bodyErr := io.ReadAll(response.Body)

	if bodyErr != nil {
		return nil, bodyErr
	}

	var jsonResponse authenticationResponse
	// fmt.Println(jsonResponse)
	unMarshallErr := json.Unmarshal(body, &jsonResponse)

	if unMarshallErr != nil {
		return nil, unMarshallErr
	}
	return jsonResponse, nil

}
