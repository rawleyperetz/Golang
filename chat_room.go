package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	//"math/rand"
	"encoding/json"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"time"
	"os"
	"errors"
	"path/filepath"
	"crypto/rand"
	"math/big"
)


// Initializing Database
var db *sql.DB

func initDB(){
	var err error
	db, err = sql.Open("sqlite3", "./chatTest.db");
	if err != nil{
		log.Fatal(err);
	}
	
}

var dhPrime, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234...", 16)

// My structs
type Room struct{
	ID string `json:"id"`
	Name string `json:"name"`
}

type Identity struct{
	ID string `json:"id"`
	Name string `json:"name"`
	Pass string `json:"passwd"`
}

type Message struct{
	ID string `json:"id"`
	RoomID string `json:"room_id"`
	Sender string `json:"sender"`
	Content string `json:"content"`
}

type RoomStatus struct{
	ID string 
	ActiveUsers int
	IPOne string
	IPTwo *string//sql.NullString
}

// Helper functions
func RandomString(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    result := make([]byte, length)
    for i := range result {
    	charsetlength := big.NewInt(int64(len(charset)));
    	n, _ := rand.Int(rand.Reader, charsetlength);
        result[i] = charset[n.Int64()];
    }
    return string(result);
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password),14);
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password));
	return err == nil;
}

func verifyRoomAccess(roomId, name, password string) (Identity, error){
	var user Identity;
	err := db.QueryRow(
		`SELECT id, name, passwd FROM rooms where id=? AND name=?;`, roomId, name).Scan(&user.ID, &user.Name, &user.Pass);
	if err != nil{
		return Identity{}, err;
	}
	//defer rows.Close(); 
	ok := CheckPasswordHash(password, user.Pass);
	if !ok {
		return Identity{}, fmt.Errorf("Invalid password");
	}
	return user, nil;
}

func getIP(r *http.Request) (string, error){
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil{
		return "", err;
	}
	if ip == "::1" {
		return "127.0.0.1", nil
	}
	return ip, nil
}

func createFile(id string, storagetype string, data string) bool{
	homeDir, err := os.UserHomeDir();
	if err != nil{
		fmt.Printf("Could not find home directory: %v\n", err);
		return false;
	}
	
	filePath := filepath.Join(homeDir, ".chatroom", id, storagetype);
	err = os.MkdirAll(filepath.Dir(filePath), 0700);
	if err != nil{
		fmt.Printf("Failed to create directory: %v\n", err);
		return false;
	}
	//filePath := "~/.chatroom/" + id + "/" + storagetype;
	if _, err := os.Stat(filePath); err == nil{
		// path/to/whatever exists
		return false;
	} else if errors.Is(err, os.ErrNotExist){
		f, err := os.Create(filePath);
		if err != nil {
			panic(err);
		}

		defer f.Close();

		_, err = f.WriteString(data);
		if err != nil{
			panic(err);
		}

		f.Sync();
		return true
		//f.Close();
	} else{
		// Schrodinger file may or may not Exist
		// Therefore, do not use !os.IsnotExist to test for file existence.
		fmt.Println("File status unknown: ", err);
		return false;
	}
}


// Endpoints
func createRoomHandler(w http.ResponseWriter, r *http.Request){
	// validate request body
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" || body.Username == "" {
		http.Error(w, "Invalid request body: name and passwd field", http.StatusBadRequest);
		return 
	}

	roomName := body.Name;

	// hash the password
	roomPass, err := HashPassword(body.Pass);
	if err != nil{
		http.Error(w, "Error hashing password", http.StatusInternalServerError);
		return;
	}
	
	active_user := 1;

	// get local or public ip address
	ip, err := getIP(r);
	if err != nil {
		http.Error(w, "IP Address Error", http.StatusInternalServerError);
		return;
	}	
	for {
		roomID := RandomString(10);
		var existing string;
		err := db.QueryRow(`SELECT id FROM rooms where id = ?;`, roomID).Scan(&existing);
		if err == sql.ErrNoRows {
				_, err = db.Exec(`INSERT INTO rooms (id, name, passwd, username, active_users, ip_one) VALUES (?,?,?,?,?,?);`, roomID, roomName, roomPass, body.Username, active_user, ip);
				if err != nil {
					http.Error(w, "Database R error", http.StatusInternalServerError);
					return;
				}
				
				// generate private key for Alice
				pMinus2 := new(big.Int).Sub(dhPrime, big.NewInt(2))
				privateKey, _ := rand.Int(rand.Reader, pMinus2);
				privateKey.Add(privateKey, big.NewInt(2));
				hexStr_private_key := privateKey.Text(16);

				// write said private key to private.key file locally
				status := createFile(roomID, "private.key", hexStr_private_key);
				if !status{
					http.Error(w, "Private Key Writing Error", http.StatusUnauthorized);
					return;
				}

				// compute generator power Alice's private key and write to dh_exchange
				two := big.NewInt(2);
				exp_value := montladder(two, privateKey, dhPrime);
				sent_value := exp_value.Text(16);
				_, err = db.Exec(`INSERT INTO dh_exchange (room_id, username, sent_value) VALUES (?,?,?);`, roomID, body.Username, sent_value);
				if err != nil{
					http.Error(w, "Database DH error", http.StatusInternalServerError);
					return;
				}
				
				w.WriteHeader(http.StatusCreated);
				w.Write([]byte(roomID));
				return;
		} 	
	}
}


func getRoomsHandler(w http.ResponseWriter, r *http.Request){
	rows, err := db.Query(`SELECT id, name FROM rooms`);
	if err != nil{
		http.Error(w, "Database error", http.StatusInternalServerError);
		return;
	}
	defer rows.Close(); 

	var rooms []Room;
	for rows.Next(){
		var room Room;
		if err:= rows.Scan(&room.ID, &room.Name); err != nil {
			http.Error(w, "Scan error", http.StatusInternalServerError);
			return;
		}
		rooms = append(rooms, room);
	}
	json.NewEncoder(w).Encode(rooms);	
}



func writeMessageHandler(w http.ResponseWriter, r *http.Request){
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Msg string `json:"msg"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" {
		http.Error(w, "Invalid request body: name field", http.StatusBadRequest);
		return; 
	}	

	roomId := r.PathValue("id");

	verifyUser, err := verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil {
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}

	for {
	msgID := RandomString(10);
	var existing string;
	err := db.QueryRow(`SELECT id FROM messages where id = ?;`, msgID).Scan(&existing);
	if err == sql.ErrNoRows {
			_, err = db.Exec(`INSERT INTO messages (id, room_id, sender, content) VALUES (?,?,?,?);`, msgID, roomId, verifyUser.Name, body.Msg);
			if err != nil {
				http.Error(w, "Database error", http.StatusInternalServerError);
				return;
			}
		
			//w.WriteHeader(http.StatusCreated);
			w.Write([]byte("Msg Sent\n"));
			return;
	 } 	
	}
}

func getMessageHandler(w http.ResponseWriter, r *http.Request){
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == ""{
		http.Error(w, "Invalid request body: name passwd field", http.StatusBadRequest);
		return;
	}

	roomId := r.PathValue("id");

	_, err = verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil{
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}

	rows, err := db.Query(`SELECT id, room_id, sender, content FROM messages where room_id=?`, roomId);
	if err != nil{
		http.Error(w, "Database error (msg)", http.StatusInternalServerError);
		return;
	}
	defer rows.Close(); 

	var msgs []Message;
	for rows.Next(){
		var msg Message;
		if err:= rows.Scan(&msg.ID, &msg.RoomID, &msg.Sender, &msg.Content); err != nil {
			http.Error(w, "Scan error", http.StatusInternalServerError);
			return;
		}
		msgs = append(msgs, msg);
	}
	if msgs == nil {
		msgs = []Message{};
	}
	w.Header().Set("Content-Type", "application/json");
	json.NewEncoder(w).Encode(msgs);
}




func waitHandler(w http.ResponseWriter, r *http.Request){
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" {
		http.Error(w, "Invalid request body: name field", http.StatusBadRequest);
		return; 
	}	

	roomId := r.PathValue("id");

	_, err = verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil {
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}

	for {
		var data RoomStatus;
		err := db.QueryRow(
		`SELECT id, active_users, ip_one, ip_two FROM rooms where id=?;`, roomId).Scan(&data.ID, &data.ActiveUsers, &data.IPOne, &data.IPTwo);

		if err != nil{
			time.Sleep(3 * time.Second);
			continue;
		}

		if data.ActiveUsers != 2 {
			time.Sleep(3 * time.Second);
			continue
		}
	// Dereferencing *dataTwo could cause a panic so we are guarding it
		if data.IPTwo == nil {
			http.Error(w, "Internal error: missing IP", http.StatusInternalServerError);
			return;
		}
		w.Write([]byte(fmt.Sprintf("Connected: %s and %s \n", data.IPOne, *data.IPTwo)));
		return;
		
	}		
}

func joinRoomHandler(w http.ResponseWriter, r *http.Request){
	// Clean Request Body
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" || body.Username{
		http.Error(w, "Invalid request body: name field", http.StatusBadRequest);
		return; 
	}	

	roomId := r.PathValue("id");

	// Verify access to Room
	_, err = verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil {
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}	

	// Get public or local IP
	ip, err := getIP(r);
	if err != nil {
		http.Error(w, "IP Address Error", http.StatusInternalServerError);
		return;
	}

	// Room is full
	var activeUsers int;
	err = db.QueryRow(`SELECT active_users FROM rooms WHERE id=?`, roomId).Scan(&activeUsers);
	if err != nil || activeUsers != 1{
		http.Error(w, "Room is full or does not exist", http.StatusForbidden);
		return;
	}

	// generate private key
	pMinus2 := new(big.Int).Sub(dhPrime, big.NewInt(2))
	privateKey, _ := rand.Int(rand.Reader, pMinus2);
	privateKey.Add(privateKey, big.NewInt(2));
	hexStr_private_key := privateKey.Text(16);

	// write private key to file
	status := createFile(roomId, "private.key", hexStr_private_key);
	if !status{
		http.Error(w, "Private Key Writing Error", http.StatusUnauthorized);
		return;
	}

	// get Alice's sent_value
	var hexStr string;
	err = db.QueryRow(`SELECT sent_value FROM dh_exchange WHERE id=? AND username != ?`, roomId, body.Username).Scan(&hexStr);
	if err != nil {
		http.Error(w, "Database DH Error 2nd Client", http.StatusUnauthorized);
		return;
	}

	n, success := new(big.Int).SetString(hexStr, 16)
	if !success{
		http.Error(w, "Hex Conversion Problem", http.StatusInternalServerError);
		return;
	}

	// Generate Bob's own and insert to dh_exchange
	exp_value := montladder(big.NewInt(2), privateKey, dhPrime);
	sent_value := exp_value.Text(16);
    _, err = db.Exec(`INSERT INTO dh_exchange (room_id, username, sent_value) VALUES (?,?,?);`, roomId, body.Username, sent_value);

	//_, err = db.Exec(`UPDATE dh_exchange SET sent_value=? WHERE id=? AND username=?;`, sent_value, roomId, body.Username);
	if err != nil{
		http.Error(w, "Database DH error", http.StatusInternalServerError);
		return;
	}

	// Alice sent_value power Bob's and write to dh_secret file
	exp_value = montladder(n, privateKey, dhPrime);
	sent_value = exp_value.Text(16);
	status = createFile(roomId, "dh_secret.key", sent_value);
	if !status{
		http.Error(w, "Dh_secret Writing Error", http.StatusUnauthorized);
		return;
	}

	// update active_users to 2
	_, err = db.Exec(`UPDATE rooms SET active_users = 2, ip_two = ? WHERE id=?;`, ip, roomId);
	if err != nil{
	 	http.Error(w, "Database Update Error", http.StatusInternalServerError);
	 	return;
	}

	w.Write([]byte(fmt.Sprintf("Connected to Room: %s on %s\n", roomId, ip)));
	return;
}

// Test out the code 
// delete and recreate the tables

func main(){
	initDB();
	
	router := http.NewServeMux();

	router.HandleFunc("POST /rooms", createRoomHandler);
	router.HandleFunc("GET /rooms", getRoomsHandler);
	router.HandleFunc("GET /rooms/{id}/wait", waitHandler);
	router.HandleFunc("POST /rooms/{id}/join", joinRoomHandler);
	router.HandleFunc("POST /rooms/{id}/messages", writeMessageHandler);
	router.HandleFunc("GET /rooms/{id}/messages", getMessageHandler);

	
	server := http.Server{Addr: ":8080", Handler: router};
	log.Println("Starting server on port :8080")
	log.Fatal(server.ListenAndServe());
}


