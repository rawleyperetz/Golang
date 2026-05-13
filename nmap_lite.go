package main

import(
	"fmt"
	"os"
	"net"
	"strconv"
	"log"
	"time"
	"flag"
)



func verify_port(port_num int ) int{
	if port_num < 0 || port_num > 65535 {
		fmt.Fprintf(os.Stderr, "Invalid Port number\n Exiting...\n");
		os.Exit(1);
	}


	//fmt.Printf("[*] Port %s is valid\n", strconv.Itoa(port_num));
	return port_num;
}

func worker(id int, host string, jobs <-chan int, results chan<- int) {
    for port := range jobs {
        address := net.JoinHostPort(host, strconv.Itoa(port))

        conn, err := net.DialTimeout("tcp", address, 50 * time.Millisecond);

        if err == nil {
            fmt.Printf("Port %d is open\n", port);
            conn.Close();
        }

        results <- port
    }
}


func main(){
	// if len(os.Args) != 3 {
	// 	fmt.Fprintf(os.Stderr, "Usage: %s <ip> <port>\n", os.Args[0]);
	// 	os.Exit(1);
	// }

	hostPtr := flag.String("host", "127.0.0.1", "a string");
	startportPtr := flag.Int("start-port", 0, "an int");
	endportPtr := flag.Int("end-port", 65535, "an int");

	flag.Parse();

	ip:= net.ParseIP(*hostPtr);
	if ip == nil {
		fmt.Println("IP address is not valid");
		os.Exit(1);
	}

	if ip.To4() != nil{
		fmt.Println("[*] IPv4 detected");
	}else{
		fmt.Println("[*] IPv6 detected");
	}

	var start_port int = verify_port(*startportPtr);
	var end_port int = verify_port(*endportPtr)

	if start_port > end_port {
		log.Fatal("Starting port must be greater than End port");
	}

// 	diff_port:= end_port - start_port;
// 	num_workers:= diff_port / 50;
// 	if num_workers < 10 {
// 	    num_workers = 10;
// 	}
// 
// 	if num_workers > 200 {
// 	    num_workers = 200
// 	}
// 	
// 	fmt.Printf("The number of workers are %d\n", num_workers);

	jobs := make(chan int, 1000);
	results := make(chan int, 1000);

	for w := 1; w <= 100; w++ {
	    go worker(w, *hostPtr, jobs, results)
	}


	for i := start_port; i <= end_port; i++ {
	    jobs <- i
	}
	close(jobs);



	total := end_port - start_port + 1
	
	for i := 0; i < total; i++ {
	    <-results
	}

	// for i:=start_port; i<=end_port; i++{
	// 	var port_string string = strconv.Itoa(i);
	// 	conn, err := net.DialTimeout("tcp", net.JoinHostPort(*hostPtr, port_string), 100000000);
	// 	if err != nil {
	//     	continue;
	// 	}
	// 	fmt.Printf("Port: %s is opened\n", port_string);
	// }

	//conn.Close();
}
