
# Chat room API
This API is a messaging backend for exactly two users with end-to-end RSA encrypted messages, Diffie-Hellman key 
exchange, RSA blinding as a countermeasure to timing attacks, and Montgomery exponentiation for modular arithmetic: 
implemented without cryptographic libraries.

The Database used in this project is SQLite3 with the following Tables and its corresponding fields:
```
Table name: Rooms
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		passwd       TEXT NOT NULL,
		username     TEXT NOT NULL,
		active_users INTEGER NOT NULL DEFAULT 1,
		ip_one       TEXT NOT NULL,
		ip_two       TEXT
```

```
Table name: Messages
		id        TEXT PRIMARY KEY,
		room_id   TEXT NOT NULL,
		sender    TEXT NOT NULL,
		content   TEXT NOT NULL,
		FOREIGN KEY (room_id) REFERENCES rooms(id)
```

```
Table name: dh_exchange 		
		room_id      TEXT NOT NULL,
		username     TEXT NOT NULL,
		sent_value   TEXT NOT NULL,
		trial_text   TEXT,
		PRIMARY KEY (room_id, username),
		FOREIGN KEY (room_id) REFERENCES rooms(id)
```
The dh_exchange table stores up intermediate Diffie-Hellman computations 


```
Table name: rsa_keys
		room_id          TEXT NOT NULL,
		username         TEXT NOT NULL,
		public_exponent  TEXT NOT NULL,
		modulus          TEXT NOT NULL,
		PRIMARY KEY (room_id, username),
		FOREIGN KEY (room_id) REFERENCES rooms(id)
```

We also generate a DhPrime (a Diffie-Hellman Prime) using a couple of helper functions (See DH topic way below (not Diffie Hellman topic))
All these tables and DhPrime are initialized at the start of the http server if they do not already exists.

## End Points
The following are the list of endpoints for this project
- POST /rooms
- GET /rooms/{id}/wait
- POST /rooms/{id}/join
- POST /rooms/{id}/messages
- GET /rooms/{id}/messages
- GET /rooms

### End Points Intricacies
We begin discussing the intricacies of each endpoint discussed above

1. POST /rooms
This end point creates a room where users can communicate. And it goes as follows.
- It validates the http Request Body but checking for fields: Name, Pass (Password), Username. All of these fields 
should be non-empty
- The password is extracted from the http Request Body and hashed using the bcrypt hashing algorithm (See HashPassword helper function)
- The ip address of the client is also extracted (see getIP helper function)
- We generate a roomID using the RandomString (helper function) and check the database (specifically the table rooms) to see
if it already exists. If it does exists, we continue our loop else we insert into our _rooms_ table the values 
roomID, name, password, active_user = 1, and ip of client. Name and passwords are extracted from the http request body

#### Diffie Hellman
If the following is unclear, see DH topic way below this document.
- We then generate a private key for this user (henceforth Alice unless stated otherwise. Sticking to Cryptograph conventions)
- We store Alice's private key locally (in this project ~/.chatroom/{roomID}/Username/private.key) using the _createFile_ helper function
- We then compute $2^{Alice's priv key} \mod DhPrime$. Turns out $2$ is used as a generator and is popularly called a
safe prime.
- The result of the above computation is converted to a hex and stored as *sent_value* field of the *dh_exchange* table along with the other 
necessary fields.
- We write to the Alice saying that *the room has been created*

2. GET /rooms/{id}/wait
After Alice has created the room, we immediately begins this endpoint to check if her friend Bob has connected. This endpoint
goes as follows
- We validate the http Request Body just as was done in the previous end-point
- We verify Alice's credentials by checking if she has access to the room (of course, she does)
- She enters a loop by continuously checking if Bob has entered the room by checking if the *active users* field of the *room* table
has been assigned to $2$. If it has, then it means Bob has entered. Also we store Bob's ip address in the *ip_two* field of the *room* table
- Here, we assume Bob has already initiated the *POST /rooms/{id}/join* endpoint (See *POST /rooms/{id}/join* before continuing)
- Alice exits the loop and fetches Bob's *sent_value* info (stored as hex in *dh_exchange*).
- Alice also fetches her *private_key* using the _openFileAndRead_ helper function.
- Both are saved as hex values so Alice converts them all (her private key and Bob's sent_value) to a big Int data type for computation
- She then computes $BobSentValue^{her private key} mod DhPrime$ to get a shared secret
- This shared secret value is converted to hex and then saved to a file locally (in this project ~/.chatroom/{roomID}/{Username}/dh_secret.key)
- She then retrieves the *trial_text* from the *dh_exchange* table (once again see *POST /rooms/{id}/join* for a better understanding)
- She un-hex-es the *trial_text* into a big Int data type then runs it against her computed shared secret using a simple XOR cipher (do not use this in a serious environment 
(See below on XOR cipher and how it works)). 
- If the result is "DH_VERIFIED"+Password then the key exchange was successful otherwise room has been compromised.

#### RSA Key Generation
- She then generates primes p, q , e (in this project e=65537 is fixed where e is the public exponent)and composite totient (\phi(n) = (p-1)*(q-1)) such that  
the gcd(e,totient) = 1. Why? so that the computation of the inverse of e is valid i.e. e*d \equiv 1 mod totient where d is the private key (not private key used in DH).
Also note that the modulus = p*q.
- She computes the private key, d (which is the inverse of e mod totient) by using the helper function _modInvEGCD_ (found in crypto.go)
- The private key, d is then converted to hex and stored in a local file (in this project ~/.chatroom/{roomID}/{Username}/rsa_priv.key)
- Then the modulus and public key, e are converted to hex and stored in the *rsa_keys* table.
- We write to Alice saying showing the two ips (hers and Bob's) are connected.

3. POST /rooms/{id}/join
So here we assume that Alice has created a room and is in the waiting endpoint. This is then is about Bob's connection
- We validate Bob's http Request just as was done with Alice in the previous end point
- We verify Bob's credentials by checking if he has access to the room 
- We extract Bob's ip address and hold onto it.
- We check if *active_users* in the *rooms* table is not 1. If it is not one, it means someone else has already entered the room (oh dearest Eve (our villain))
- We then generate the Diffie Hellman private key for Bob, write said private key to local file, get Alice's *sent_value*, generate Bob's $2^{his private key} mod DhPrime$ and
send to *dh_exchange* table as *sent_value*, compute shared secret using Alice's *sent_value* that is $AliceSentValue^{Bob's private key} mod DhPrime)$ and store share secret in
local file *dh_secret.key* (this process is the same as in the subtopic above (Diffie-Hellman)).
- We update *active_users* to 2
- Then we compute a ciphertext using a simple XOR Cipher on the message "DH_VERIFIED:"+Password, convert to hex and store in *trial_text* of *dh_exchange* table
- We do the same RSA key generation (see subtopic above) for Bob.
- We then tell Bob that he has connected to the Room with Id on IP address bla bla bla.



4. POST /rooms/{id}/messages
In this endpoints, we assume either Alice or Bob wishes to write a message to their partner. This end point goes as follows
- We validate the http Request Body just as was done in the previous end-point above.
- We verify user's credential by checking if he or she has access to the room.
