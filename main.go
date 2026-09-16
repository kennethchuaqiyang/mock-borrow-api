
// Mock API server
//
// Endpoints:
//   GET  /api  - fetch user metadata (username, location, salary) + cache block
//   POST /api  - borrow request, returns amount owed / whether borrowing is allowed
//
// Run:
//   go run main.go
//   (listens on :8080)
//
// Data is stored in-memory in `userStore`. Swap `userStore` for a DB or Excel-backed
// repository later — the handlers only depend on the Get/Save methods below, so the
// rest of the code doesn't need to change.
 
package main
 
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
	"os"
)
 
// ---------- Config ----------
 
const secretSalt = "supersecretkey" // replace with a real secret (env var) in production
const borrowLimit = 200.0           // "if new amount owed above 200, no else yes"
 
// ---------- Data model ----------
 
// User is the mock "database row" for a user. Swap this out for a real DB/Excel
// lookup later; keep the same fields so handlers don't need to change.
type User struct {
	UserID     int
	Username   string
	Location   string
	Salary     float64
	AmountOwed float64
}
 
// userStore is a thread-safe in-memory store. Replace with a DB/Excel-backed
// implementation later by giving that implementation the same Get/Save methods.
type userStore struct {
	mu    sync.Mutex
	users map[int]*User
}
 
func newUserStore() *userStore {
	s := &userStore{users: make(map[int]*User)}
	// seed a couple of example users
	s.users[1] = &User{UserID: 1, Username: "john", Location: "Singapore", Salary: 5000, AmountOwed: 0}
	s.users[2] = &User{UserID: 2, Username: "mary", Location: "Kuala Lumpur", Salary: 4200, AmountOwed: 150}
	return s
}
 
// Get returns the user, creating a default record if one doesn't exist yet
// (handy for a mock server where any userid should "just work").
func (s *userStore) Get(id int, username, location string) *User {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		u = &User{
			UserID:     id,
			Username:   username,
			Location:   location,
			Salary:     3000, // default mock salary
			AmountOwed: 0,
		}
		s.users[id] = u
	}
	return u
}
 
func (s *userStore) Save(u *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.UserID] = u
}
 
var store = newUserStore()
 
// ---------- Shared helpers ----------
 
// generateSecretKey = sha256(id + username + currentDate + secret)
func generateSecretKey(id int, username string) string {
	date := time.Now().Format("2006-01-02")
	raw := fmt.Sprintf("%d%s%s%s", id, username, date, secretSalt)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
 
// readCommonHeaders pulls the two required request headers.
// Header names are treated case-insensitively (Go's http package already
// normalizes them), so "browser type" -> "X-Browser-Type" and
// "admin flag" -> "X-Admin-Flag".
func readCommonHeaders(r *http.Request) (browserType string, adminFlag bool) {
	browserType = r.Header.Get("X-Browser-Type")
	adminFlag, _ = strconv.ParseBool(r.Header.Get("X-Admin-Flag")) // defaults to false if missing/invalid
	return
}
 
func writeCommonResponseHeaders(w http.ResponseWriter, browserType string, id int, username string) {
	w.Header().Set("X-Browser", browserType)
	w.Header().Set("X-Secret-Key", generateSecretKey(id, username))
	w.Header().Set("Content-Type", "application/json")
}
 
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
 
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
 
// ---------- GET /api ----------
 
type getMetadata struct {
	Username string  `json:"username"`
	Location string  `json:"location"`
	Salary   float64 `json:"salary"`
}
 
type getCache struct {
	Username     string `json:"username"`
	UserIdentity int    `json:"user_identity"`
	Salary       float64 `json:"salary"`
}
 
type getResponse struct {
	Metadata getMetadata `json:"metadata"`
	Cache    getCache    `json:"cache"`
}
 
func handleGet(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is supported on /api/user")
		return
	}
	q := r.URL.Query()
	username := q.Get("username")
	location := q.Get("location")
	userIDStr := q.Get("userid")
 
	if username == "" || userIDStr == "" {
		writeError(w, http.StatusBadRequest, "username and userid are required query parameters")
		return
	}
 
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "userid must be an integer")
		return
	}
 
	browserType, adminFlag := readCommonHeaders(r)
	_ = adminFlag // not used for business logic yet; wire in access rules here if needed
 
	user := store.Get(userID, username, location)
 
	writeCommonResponseHeaders(w, browserType, user.UserID, user.Username)
	writeJSON(w, http.StatusOK, getResponse{
		Metadata: getMetadata{
			Username: user.Username,
			Location: user.Location,
			Salary:   user.Salary,
		},
		Cache: getCache{
			Username:     user.Username,
			UserIdentity: user.UserID,
			Salary:       user.Salary,
		},
	})
}
 
// ---------- POST /api ----------
 
type postRequest struct {
	Username       string  `json:"username"`
	Location       string  `json:"location"`
	UserID         int     `json:"userid"`
	AmountToBorrow float64 `json:"amount_to_borrow"`
}
 
type postMetadata struct {
	Username          string  `json:"username"`
	Location          string  `json:"location"`
	AmountOwed        float64 `json:"amount_owed"`
	AmountAlteredBy   float64 `json:"amount_altered_to_borrow"`
	AllowedToBorrow   bool    `json:"allowed_to_borrow"`
}
 
type postResponse struct {
	Metadata postMetadata `json:"metadata"`
}
 
func handlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST is supported on /api/borrow")
		return
	}
	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
 
	browserType, adminFlag := readCommonHeaders(r)
	_ = adminFlag
 
	user := store.Get(req.UserID, req.Username, req.Location)
 
	proposedOwed := user.AmountOwed + req.AmountToBorrow
	allowed := proposedOwed <= borrowLimit // "above 200 -> no, else yes"
 
	amountAltered := 0.0
	finalOwed := user.AmountOwed
	if allowed {
		amountAltered = req.AmountToBorrow
		finalOwed = proposedOwed
		user.AmountOwed = finalOwed
		store.Save(user)
	}
 
	writeCommonResponseHeaders(w, browserType, user.UserID, user.Username)
	writeJSON(w, http.StatusOK, postResponse{
		Metadata: postMetadata{
			Username:        user.Username,
			Location:        user.Location,
			AmountOwed:      finalOwed,
			AmountAlteredBy: amountAltered,
			AllowedToBorrow: allowed,
		},
	})
}
 
// ---------- Router ----------
 
// func apiHandler(w http.ResponseWriter, r *http.Request) {
// 	switch strings.ToUpper(r.Method) {
// 	case http.MethodGet:
// 		handleGet(w, r)
// 	case http.MethodPost:
// 		handlePost(w, r)
// 	default:
// 		writeError(w, http.StatusMethodNotAllowed, "only GET and POST are supported on /api")
// 	}
// }
 
func main() {
	http.HandleFunc("/api/user", handleGet)
	http.HandleFunc("/api/borrow", handlePost)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // fallback for local development
	}
	addr := ":" + port
	log.Printf("mock server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}