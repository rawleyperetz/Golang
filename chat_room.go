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
	"encoding/hex"
	"strings"
)


// Initializing Database
var db *sql.DB

func initDB(){
	var err error
	db, err = sql.Open("sqlite3", "./chatTest.db");
	if err != nil{
		log.Fatal(err);
	}

	createRoomsSQL := `CREATE TABLE IF NOT EXISTS rooms(
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		passwd       TEXT NOT NULL,
		username     TEXT NOT NULL,
		active_users INTEGER NOT NULL DEFAULT 1,
		ip_one       TEXT NOT NULL,
		ip_two       TEXT
	);`

	_, err = db.Exec(createRoomsSQL);
	if err != nil{
		log.Fatalf("Error creating Rooms table: %q\n", err);
	}

	messagesSQL := `CREATE TABLE IF NOT EXISTS messages (
		id        TEXT PRIMARY KEY,
		room_id   TEXT NOT NULL,
		sender    TEXT NOT NULL,
		content   TEXT NOT NULL,
		FOREIGN KEY (room_id) REFERENCES rooms(id)
	);`

	_, err = db.Exec(messagesSQL);
	if err != nil{
		log.Fatalf("Error creating messages table: %q", err);
	}

	dhSQL := `CREATE TABLE IF NOT EXISTS dh_exchange(
		room_id      TEXT NOT NULL,
		username     TEXT NOT NULL,
		sent_value   TEXT NOT NULL,
		trial_text   TEXT,
		PRIMARY KEY (room_id, username),
		FOREIGN KEY (room_id) REFERENCES rooms(id)
	);`

	_, err = db.Exec(dhSQL);
	if err != nil{
		log.Fatalf("Error creating DH table: %q", err);
	}

	rsaSQL := `CREATE TABLE IF NOT EXISTS rsa_keys(
		room_id          TEXT NOT NULL,
		username         TEXT NOT NULL,
		public_exponent  TEXT NOT NULL,
		modulus          TEXT NOT NULL,
		PRIMARY KEY (room_id, username),
		FOREIGN KEY (room_id) REFERENCES rooms(id)
	);`

	_, err = db.Exec(rsaSQL);
	if err != nil{
		log.Fatalf("Error creating rsa_keys table: %q", err);
	}
}

// RFC 3526 2048-bit prime
// https://datatracker.ietf.org/doc/html/rfc3526
// too heavy for my 8gb ram
// var dhPrime, _ = new(big.Int).SetString(
//     "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1"+
//     "29024E088A67CC74020BBEA63B139B22514A08798E3404DD"+
//     "EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245"+
//     "E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
//     "EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D"+
//     "C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F"+
//     "83655D23DCA3AD961C62F356208552BB9ED529077096966D"+
//     "670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
//     "E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9"+
//     "DE2BCBF6955817183995497CEA956AE515D2261898FA0510"+
//     "15728E5A8AACAA68FFFFFFFFFFFFFFFF", 16);

var dhPrime = genDHPrime(60, 20);


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

type RsaInfo struct {
	PublicExponent string
	Modulus string
}


// Helper functions
func genDHPrime(bitLength int, rounds int) *big.Int{
	one := big.NewInt(1);
	for {
		p := generatePrime(bitLength, rounds);
		q := new(big.Int);
		q.Sub(p, one)
		q.Rsh(q, 1);
		s, d := prepCandidate(q);
		if MillerRabin(s, d, rounds){
			return p;
		}
	}
}


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

func createFile(id string, storagetype string, username string, data string) bool{
	homeDir, err := os.UserHomeDir();
	if err != nil{
		fmt.Printf("Could not find home directory: %v\n", err);
		return false;
	}
	
	filePath := filepath.Join(homeDir, ".chatroom", id, username, storagetype);
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

func openFileAndRead(id string, storagetype string, username string) (string, bool) {
	homeDir, err := os.UserHomeDir();
	if err != nil{
		fmt.Printf("Could not find Home directory: %v\n", err);
		return "", false;
	}

	filePath := filepath.Join(homeDir, ".chatroom", id, username, storagetype);
	fileData, err := os.ReadFile(filePath);
	if err != nil{
		fmt.Println("Error: ", err);
		return "", false;
	}

	// Fast-trim trailing newlines  without allocating new memory
	fileStr := strings.TrimRight(string(fileData), "\r\n")
	return fileStr, true
	// fileData = bytes.TrimRight(fileData, "\r\n");
	// return string(fileData), true;
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
				status := createFile(roomID, "private.key", body.Username, hexStr_private_key);
				if !status{
					http.Error(w, "Private Key Writing Error", http.StatusUnauthorized);
					return;
				}

				// compute generator power Alice's private key and write to dh_exchange
				two := big.NewInt(2);
				exp_value := montLadder(two, privateKey, dhPrime);
				sent_value := exp_value.Text(16);
				_, err = db.Exec(`INSERT INTO dh_exchange (room_id, username, sent_value) VALUES (?,?,?);`, roomID, body.Username, sent_value);
				if err != nil{
					http.Error(w, "Database DH RUS error", http.StatusInternalServerError);
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
	// validate request Body
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Msg string `json:"msg"`
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" || body.Msg == "" || body.Username == ""{
		http.Error(w, "Invalid request body: name field", http.StatusBadRequest);
		return; 
	}	

	roomId := r.PathValue("id");

	// Verify user's access to the Room
	verifyUser, err := verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil {
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}


	for {
	msgID := RandomString(10);
	var existing string;
	err := db.QueryRow(`SELECT id FROM messages WHERE id = ?;`, msgID).Scan(&existing);
	if err == sql.ErrNoRows {
			// get other user's rsa pub exponent and modulus
			var rsa_data RsaInfo;
			err = db.QueryRow(`SELECT public_exponent, modulus FROM rsa_keys WHERE room_id=? AND username <> ?;`, roomId, body.Username).Scan(&rsa_data.PublicExponent, &rsa_data.Modulus);
			if err == sql.ErrNoRows {
				http.Error(w, "RSA pub modulus retrieval failed", http.StatusInternalServerError);
				return;
			}

// 			getPub, _ := hex.DecodeString(rsa_data.PublicExponent);
// 			getModulus, _ := hex.DecodeString(rsa_data.Modulus);
// 
// 			pubExp := new(big.Int).SetBytes(getPub);
// 			modulus := new(big.Int).SetBytes(getModulus);
			pubExp, _ := new(big.Int).SetString(rsa_data.PublicExponent, 16)
			modulus, _ := new(big.Int).SetString(rsa_data.Modulus, 16)


			m := new(big.Int).SetBytes([]byte(body.Msg));

			// debugging
			// fmt.Printf("rsa data Public exponent: %s\n", rsa_data.PublicExponent);
			// fmt.Printf("ENCRYPT m: %s\n", m.Text(16))
			// fmt.Printf("ENCRYPT pubExp: %s\n", pubExp.Text(16))
			// fmt.Printf("ENCRYPT modulus: %s\n", modulus.Text(16))
			// fmt.Printf("ENCRYPT m < modulus: %v\n", m.Cmp(modulus) < 0)


			
			c := montLadder(m, pubExp, modulus);

			//debugging
			//fmt.Printf("ENCRYPT c: %s\n", c.Text(16))
			
			ciphertext := c.Text(16);
			//debugging
			//fmt.Printf("ENCRYPT stored hex: %s\n", ciphertext)
			
			_, err = db.Exec(`INSERT INTO messages (id, room_id, sender, content) VALUES (?,?,?,?);`, msgID, roomId, verifyUser.Name, ciphertext);
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
	// validate request Body
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" || body.Username == ""{
		http.Error(w, "Invalid request body: name passwd field", http.StatusBadRequest);
		return;
	}

	roomId := r.PathValue("id");

	// Verify User's room access
	_, err = verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil{
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}

	// get all rows from user's partner in crime
	rows, err := db.Query(`SELECT id, room_id, sender, content FROM messages where room_id=? AND sender <> ?;`, roomId, body.Username);
	if err != nil{
		http.Error(w, "Database error (msg)", http.StatusInternalServerError);
		return;
	}
	defer rows.Close(); 

	// get user's modulus from rsa table
	//var hexModulus, hexPubExp string;
	var rsa_data RsaInfo;
	err = db.QueryRow(`SELECT public_exponent, modulus FROM rsa_keys where room_id=? AND username = ?;`, roomId, body.Username).Scan(&rsa_data.PublicExponent, &rsa_data.Modulus);
	if err != nil{
		http.Error(w, "Missing my Modulus", http.StatusInternalServerError);
		return;
	}

	modulus, _ := new(big.Int).SetString(rsa_data.Modulus, 16);
	pubExp, _ := new(big.Int).SetString(rsa_data.PublicExponent, 16);	
	// Load rsa priv key from file
	//fmt.Printf("Modulus: %d\n", modulus)
	
	privHex, ok := openFileAndRead(roomId, "rsa_priv.key", body.Username);
	if !ok{
		http.Error(w, "Could not Open RSA priv key file", http.StatusUnauthorized);
		return;
	}

	//fmt.Printf("The rsa key is: %x\n", privHex);
	privKey, _ := new(big.Int).SetString(privHex, 16);
	//fmt.Printf("Priv key: %d\n", privKey)

	// if modulus.Cmp(privKey) > 0{
	// 	fmt.Printf("Modulus is greater\n");
	// }else{
	// 	fmt.Printf("Modulus is smaller\n");
	// }
	
	one := big.NewInt(1);
	gcd := new(big.Int);
	
	var msgs []Message;
	for rows.Next(){
		var msg Message;
		if err:= rows.Scan(&msg.ID, &msg.RoomID, &msg.Sender, &msg.Content); err != nil {
			http.Error(w, "Scan error", http.StatusInternalServerError);
			return;
		}
		// decrypt rsa content for other user because they used your public key
		// recover ciphertext as big.Int
		cipherBytes, _ := hex.DecodeString(msg.Content)
		c := new(big.Int).SetBytes(cipherBytes)
// 
		// fmt.Printf("c: %s\n", c.Text(16))
		// fmt.Printf("c < modulus: %v\n", c.Cmp(modulus) < 0)
		
 		//m := montLadder(c, privKey, modulus);


		// debugging
//  		fmt.Printf("DECRYPT modulus: %s\n", modulus.Text(16))
//  		fmt.Printf("DECRYPT privKey: %s\n", privKey.Text(16))
//  		fmt.Printf("DECRYPT retrieved hex: %s\n", msg.Content)
//  		fmt.Printf("DECRYPT c: %s\n", c.Text(16))
//  		fmt.Printf("DECRYPT c < modulus: %v\n", c.Cmp(modulus) < 0)
// 
//  		fmt.Printf("DECRYPT m: %s\n", m.Text(16))
//  		fmt.Printf("DECRYPT result: %s\n", string(m.Bytes()))
// 		// generate r coprime with modulus
		// here we do the rsa blinding
		var rBlind *big.Int
		for {
			rBlind, _ = rand.Int(rand.Reader, modulus)
			
			rBlind.Add(rBlind, one);
			gcd.GCD(nil, nil, rBlind, modulus)
			if gcd.Cmp(one) == 0 {
				break
			}
		}

		// blind: c' = c * r^e mod n
		rE := montLadder(rBlind, pubExp, modulus)
		cBlinded := new(big.Int).Mul(c, rE)
		cBlinded.Mod(cBlinded, modulus)

		// decrypt: m' = (c')^d mod n
		mBlinded := montLadder(cBlinded, privKey, modulus)

		// unblind: m = m' * r^-1 mod n
		rInv := modInvEGCD(rBlind, modulus)
		m := new(big.Int).Mul(mBlinded, rInv)
		m.Mod(m, modulus)
// 
// 		// convert back to string
// 		msg.Content = string(m.Bytes()) 
// 		fmt.Printf("Decrypted Hex: %x\n", m.Bytes()) 
		
		
		msg.Content = string(m.Bytes())
		
		//fmt.Printf("variable amount is of type %T \n", msg.Content);
		msgs = append(msgs, msg);
	}
	if msgs == nil {
		msgs = []Message{};
	}
	w.Header().Set("Content-Type", "application/json");
	json.NewEncoder(w).Encode(msgs);
}




func waitHandler(w http.ResponseWriter, r *http.Request){
	// validate Request Body
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" || body.Username == "" {
		http.Error(w, "Invalid request body: name field", http.StatusBadRequest);
		return; 
	}	

	roomId := r.PathValue("id");

	// Verify Alice User's access to Room
	_, err = verifyRoomAccess(roomId, body.Name, body.Pass);
	if err != nil {
		http.Error(w, "Access Denied", http.StatusUnauthorized);
		return;
	}

	var data RoomStatus;
	for {
		
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
		break;
		
	}		

	// Fetch Bob's sent_value
	var bobHex string;
	err = db.QueryRow(`SELECT sent_value FROM dh_exchange WHERE room_id=? AND username <> ?;`, roomId, body.Username).Scan(&bobHex);
	if err != nil{
		http.Error(w, "Error Retrieving SV", http.StatusInternalServerError);
		return;
	}

	// Load Alice's private key from file
	privHex, ok := openFileAndRead(roomId, "private.key", body.Username);
	if !ok{
		http.Error(w, "Could not Open Alice priv key file", http.StatusUnauthorized);
		return;
	}

	// Compute shared secret
	bobSentValue, _:= new(big.Int).SetString(bobHex, 16);
	privKey, _ := new(big.Int).SetString(privHex, 16);
	sharedSecret := montLadder(bobSentValue, privKey, dhPrime);
	dhSecretHex := sharedSecret.Text(16);

	// Save to dh_secret file
	status := createFile(roomId, "dh_secret.key", body.Username, dhSecretHex);
	if !status{
		http.Error(w, "Private Key Writing Error", http.StatusUnauthorized);
		return;
	}

	var EncodedhexStr string;
	err = db.QueryRow(`SELECT trial_text FROM dh_exchange WHERE room_id=? AND username <> ?`, roomId, body.Username).Scan(&EncodedhexStr);
	if err != nil {
		http.Error(w, "Database DH Error 2nd Client", http.StatusUnauthorized);
		return;
	}

	decoded, _:= hex.DecodeString(EncodedhexStr);
	// hexInDH, ok := openFileAndRead(roomId, "dh_secret.key");
	// if !ok{
	// 	http.Error(w, "Error reading DH file", http.StatusInternalServerError);
	// 	return;
	// }
	
	// opening dh_secret.key file
	msg := simpleXORCipher(string(decoded), dhSecretHex);
	w.Write([]byte(fmt.Sprintf("Msg Decoded: %s\n", hex.EncodeToString([]byte(msg)))));

	expected := "DH_VERIFIED:" + body.Pass;
	if msg != expected{
		http.Error(w, "DH verification failed -- room may be compromised", http.StatusUnauthorized);
		return;
	}


	// rsa keygen
	p := new(big.Int);
	q := new(big.Int);
	pubExp := big.NewInt(65537);
	//fmt.Printf("pubExp hex wait: %s\n", pubExp.Text(16))
	totient := new(big.Int);
	one := big.NewInt(1);
	for{
		p = generatePrime(256, 20);
		q = generatePrime(256, 20);
		pMinus1 := new(big.Int).Sub(p, big.NewInt(1));
		qMinus1 := new(big.Int).Sub(q, big.NewInt(1));
		
		totient = new(big.Int).Mul(pMinus1, qMinus1);
		// if new(big.Int).Mod(totient, pubExp).Cmp(big.NewInt(0)) != 0 {
		//     break;
		// }
		// Calculate the GCD of pubExp and totient
		gcd := new(big.Int).GCD(nil, nil, pubExp, totient)
		    
		    // If GCD is 1, they are coprime! We can break the loop.
		if gcd.Cmp(one) == 0 {
		        break
		}
		
	}
	 
	modulus := new(big.Int).Mul(p,q);
	privkey := modInvEGCD(pubExp, totient);
	//fmt.Printf("Alice priv key: %d\n", privkey)
	hexPrivKey := privkey.Text(16);
	status = createFile(roomId, "rsa_priv.key", body.Username, hexPrivKey);
	if !status{
		http.Error(w, "RSA priv key writing Error", http.StatusUnauthorized);
		return;
	}

	hexModulus := modulus.Text(16);
	hexPubExp := pubExp.Text(16);

   _, err = db.Exec(`INSERT INTO rsa_keys (room_id, username, public_exponent, modulus) VALUES (?,?,?,?);`, roomId, body.Username, hexPubExp ,hexModulus);

	//_, err = db.Exec(`UPDATE dh_exchange SET sent_value=? WHERE id=? AND username=?;`, sent_value, roomId, body.Username);
	if err != nil{
		http.Error(w, "Database RS Insert Error", http.StatusInternalServerError);
		return;
	}
		
	w.Write([]byte(fmt.Sprintf("Connected: %s and %s \n", data.IPOne, *data.IPTwo)));
	return;
}




func joinRoomHandler(w http.ResponseWriter, r *http.Request){
	// Clean Request Body
	var body struct {
		Name string `json:"name"`
		Pass string `json:"passwd"`
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Pass == "" || body.Username == ""{
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
	status := createFile(roomId, "private.key", body.Username, hexStr_private_key);
	if !status{
		http.Error(w, "Private Key Writing Error", http.StatusUnauthorized);
		return;
	}

	// get Alice's sent_value
	var hexStr string;
	err = db.QueryRow(`SELECT sent_value FROM dh_exchange WHERE room_id=? AND username <> ?;`, roomId, body.Username).Scan(&hexStr);
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
	exp_value := montLadder(big.NewInt(2), privateKey, dhPrime);
	sent_value := exp_value.Text(16);
    _, err = db.Exec(`INSERT INTO dh_exchange (room_id, username, sent_value) VALUES (?,?,?);`, roomId, body.Username, sent_value);

	//_, err = db.Exec(`UPDATE dh_exchange SET sent_value=? WHERE id=? AND username=?;`, sent_value, roomId, body.Username);
	if err != nil{
		http.Error(w, "Database DH error", http.StatusInternalServerError);
		return;
	}

	// Alice sent_value power Bob's and write to dh_secret file
	exp_value = montLadder(n, privateKey, dhPrime);
	sent_value = exp_value.Text(16);
	status = createFile(roomId, "dh_secret.key", body.Username, sent_value);
	if !status{
		http.Error(w, "DH_secret Writing Error", http.StatusUnauthorized);
		return;
	}

	// update active_users to 2
	_, err = db.Exec(`UPDATE rooms SET active_users = 2, ip_two = ? WHERE id=?;`, ip, roomId);
	if err != nil{
	 	http.Error(w, "Database Update Error", http.StatusInternalServerError);
	 	return;
	}

	// sending trial
	ciphertext := simpleXORCipher("DH_VERIFIED:"+body.Pass, sent_value);
	ciphertextToHexEncoded := hex.EncodeToString([]byte(ciphertext));
	_, err = db.Exec(`UPDATE dh_exchange SET trial_text=? WHERE username=? AND room_id=?;`, ciphertextToHexEncoded, body.Username, roomId);
	if err != nil{
		http.Error(w, "Database DH TT error", http.StatusInternalServerError);
		return;
	}

	// rsa keygen
	p := new(big.Int);
	q := new(big.Int);
	pubExp := big.NewInt(65537);
	//fmt.Printf("pubExp hex join: %s\n", pubExp.Text(16))
	totient := new(big.Int)
	for{
		p = generatePrime(256, 20);
		q = generatePrime(256, 20);
		pMinus1 := new(big.Int).Sub(p, big.NewInt(1));
		qMinus1 := new(big.Int).Sub(q, big.NewInt(1));
		
		totient = new(big.Int).Mul(pMinus1, qMinus1);
		// if new(big.Int).Mod(totient, pubExp).Cmp(big.NewInt(0)) != 0 {
		//     break;
		// }
		gcd := new(big.Int).GCD(nil, nil, pubExp, totient)
		if gcd.Cmp(big.NewInt(1)) == 0 {
		    break
		}
	}
	 
	modulus := new(big.Int).Mul(p,q);
	privkey := modInvEGCD(pubExp, totient);
	//fmt.Printf("Bob priv key: %d\n", privkey)
	hexPrivKey := privkey.Text(16);
	status = createFile(roomId, "rsa_priv.key", body.Username, hexPrivKey);
	if !status{
		http.Error(w, "RSA priv key writing Error", http.StatusUnauthorized);
		return;
	}

	hexModulus := modulus.Text(16);
	hexPubExp := pubExp.Text(16);

   _, err = db.Exec(`INSERT INTO rsa_keys (room_id, username, public_exponent, modulus) VALUES (?,?,?,?);`, roomId, body.Username, hexPubExp ,hexModulus);

	//_, err = db.Exec(`UPDATE dh_exchange SET sent_value=? WHERE id=? AND username=?;`, sent_value, roomId, body.Username);
	if err != nil{
		http.Error(w, "Database RS Insert Error", http.StatusInternalServerError);
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
