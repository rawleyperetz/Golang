package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"math/rand"
	"encoding/json"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"time"
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
        result[i] = charset[rand.Intn(len(charset))]
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


// Endpoints
func createRoomHandler(w http.ResponseWriter, r *http.Request){
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" {
		http.Error(w, "Invalid request body: name and passwd field", http.StatusBadRequest);
		return 
	}

	roomName := body.Name;
	roomPass, err := HashPassword(body.Pass);
	if err != nil{
		http.Error(w, "Error hashing password", http.StatusInternalServerError);
		return;
	}
	active_user := 1;
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
				_, err = db.Exec(`INSERT INTO rooms (id, name, passwd, active_users, ip_one) VALUES (?,?,?,?,?);`, roomID, roomName, roomPass, active_user, ip);
				if err != nil {
					http.Error(w, "Database error", http.StatusInternalServerError);
					return;
				}
				w.WriteHeader(http.StatusCreated);
				//http.Redirect(w, r, ,http.StatusFound);
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


