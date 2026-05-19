package main

import (
	//"fmt"
	"crypto/rand"
	"slices"
	"math/big"
	//"encoding/hex"
	//"crypto/rand"
)

func fromBinary(bits []int) *big.Int{
	result := big.NewInt(0);
	length_bits := len(bits);
	for i:=0; i < length_bits; i++{
		if bits[i] == 1 {
			bit := new(big.Int).Lsh(big.NewInt(1), uint(i));
			result.Or(result, bit);
			//result |= (1 << i);
		}
	}
	
	return result;
}


func generateCandidatePrime(bitlen int) *big.Int{
	//length := *bitlen;
	
	bits := []int{1};

	for i:= 0; i < (bitlen - 2); i++ {
		//bits = append(bits, rand.Intn(2));
		bit, err := rand.Int(rand.Reader, big.NewInt(2));
		if err != nil{
			panic(err);
		}
		bits = append(bits, int(bit.Int64()));	
		//n, _ := rand.Int(rand.Reader, 2)
		//bits = append(bits,n);
	}
	
	bits = append(bits, 1);
	
	return fromBinary(bits);
}

func prepCandidate(candidate *big.Int) (*big.Int, *big.Int){
	s := big.NewInt(0);
	//d := candidate - 1;
	d := new(big.Int).Sub(candidate, big.NewInt(1));
	// for (d % 2) == 0 {
	// 	d >>= 1;
	// 	s += 1;
	// }
	two := big.NewInt(2);
	mod := new(big.Int);
	for mod.Mod(d, two); mod.Cmp(big.NewInt(0)) == 0; mod.Mod(d, two){
		d.Rsh(d,1);
		s.Add(s, big.NewInt(1));
	}
	return s, d;
}

func toBinary(num *big.Int) []*big.Int {
	var bits []*big.Int;
	// if num == 0 {
	// 	return []big.Int{0};
	// }
	// for num > 0 {
	// 	bits = append(bits, num%2);
	// 	num = num >> 1;
	// }
	zero := big.NewInt(0);
	two := big.NewInt(2);
	if num.Cmp(zero) == 0 {
		return []*big.Int{big.NewInt(0)};
	}
	n := new(big.Int).Set(num);
	for n.Cmp(zero) > 0 {
		mod := new(big.Int);
		n.DivMod(n, two, mod);
		bits = append(bits, new(big.Int).Set(mod))
	}
	return bits;
}

func computeR(modulus *big.Int) *big.Int{
	R := big.NewInt(1);
	for R.Cmp(modulus) <= 0 {
		R.Lsh(R, 1);
	}
	return R;
}
// compute inverse using Newton raphson lifting
func modinv(n *big.Int, R *big.Int) *big.Int {
	x := big.NewInt(1);
	two := big.NewInt(2);
	mask := new(big.Int).Sub(R, big.NewInt(1));
	for i:= 0; i < 64; i++{
		nx := new(big.Int).Mul(n,x);
		nx.Mod(nx, R);
		t := new(big.Int).Sub(two, nx);
		x.Mul(x,t);
		x.And(x, mask);
	}
	return x;
}
	// for i:=0; i<6; i++ {
	// 	x = x * (2 - n * x);
	// }	
	// return x & (R - 1);


func montgomeryRedc(T *big.Int, modulus *big.Int) *big.Int {
	//R :=  big.NewInt(1);
	R := computeR(modulus);
	mask := new(big.Int).Sub(R, big.NewInt(1));
	nInv := modinv(modulus, R);
	nPrime := new(big.Int).Sub(R, nInv);
	nPrime.And(nPrime, mask);

	m := new(big.Int).Mul(T, nPrime)
	m.And(m, mask);

	t:= new(big.Int).Mul(m, modulus);
	t.Add(t, T)
	t.Rsh(t, uint(R.BitLen()-1));

	if t.Cmp(modulus) >= 0 {
		t.Sub(t, modulus)
	}
	return t;
}


func montgomeryMult(a *big.Int, b *big.Int, modulus *big.Int) *big.Int {

	//R := big.NewInt(1);
	R := computeR(modulus);
	aMont := new(big.Int).Mul(a,R);
	aMont.Mod(aMont, modulus);

	bMont := new(big.Int).Mul(b, R);
	bMont.Mod(bMont, modulus);

	product := new(big.Int).Mul(aMont, bMont);
	return montgomeryRedc(product, modulus);
}




func montLadder(g *big.Int, k *big.Int, n *big.Int) *big.Int{
	r0 := montgomeryMult(big.NewInt(1), big.NewInt(1), n);
	r1 := montgomeryMult(g, big.NewInt(1), n);

	binK := toBinary(k);
	slices.Reverse(binK);

	for _, bit := range binK{
		if bit.Cmp(big.NewInt(0)) == 0 {
			product := new(big.Int).Mul(r1, r0);
			r1 = montgomeryRedc(product, n);
			product = new(big.Int).Mul(r0, r0);
			r0 = montgomeryRedc(product, n);
		}else{
			product := new(big.Int).Mul(r1, r0);
			r0 = montgomeryRedc(product, n);
			product = new(big.Int).Mul(r1, r1);
			r1 = montgomeryRedc(product, n);			
		}
	}
	return montgomeryRedc(r0, n);
}



func MillerRabin(s *big.Int, d *big.Int, rounds int) bool{
	nMinusOne := new(big.Int).Lsh(big.NewInt(1), uint(s.Uint64()));
	nMinusOne.Mul(nMinusOne, d);
	n := new(big.Int).Add(nMinusOne, big.NewInt(1));

	nMinus3 := new(big.Int).Sub(n, big.NewInt(3));

	for i:= 0; i < rounds; i++{
		//aInt, _:= rand.Int(rand.New(rand.NewSource(rand.Int63())), nMinus3);
		aInt, err := rand.Int(rand.Reader, nMinus3);
		if err != nil {
			panic(err);
		}
		a := new(big.Int).Add(aInt, big.NewInt(2));

		x:= montLadder(a, d, n);

		one := big.NewInt(1);
		if x.Cmp(one) == 0 || x.Cmp(nMinusOne) == 0{
			continue
		}

		composite := true;
		r := big.NewInt(1)
		for r.Cmp(s) < 0 {
			x = montLadder(x, big.NewInt(2), n);
			if x.Cmp(nMinusOne) == 0{
				composite = false;
				break;
			}
			r.Add(r, big.NewInt(1));
		}

		if composite{
			return false
		}
	}

	return true;
}





func generatePrime(bitLength int, rounds int) *big.Int{
	for {
		candidatePrime := generateCandidatePrime(bitLength);
		s, d := prepCandidate(candidatePrime);
		if MillerRabin(s, d, rounds){
			//possiblePrime =  ((1 << s) * d) + 1;
			return candidatePrime; 
		}
	}
}


func simpleXORCipher(msgString string, dhHexStr string) string{
	lengthOfHexStr := len(dhHexStr);
	lengthOfMsgStr := len(msgString);

	// here, we allocate a byte slice of target length
	resultBytes := make([]byte, lengthOfMsgStr);
	
	for i:=0; i < lengthOfMsgStr; i++{
		resultBytes[i] =  msgString[i] ^ dhHexStr[i % lengthOfHexStr];
	}
	
	return string(resultBytes);
}

func extGCD(a *big.Int, b *big.Int) (*big.Int, *big.Int, *big.Int) {
    if a.Cmp(big.NewInt(0)) == 0 {
        return b, big.NewInt(0), big.NewInt(1)
    }
    mod := new(big.Int).Mod(b, a)
    g, x, y := extGCD(mod, a)
    newX := new(big.Int).Sub(y, new(big.Int).Mul(new(big.Int).Div(b, a), x))
    return g, newX, x
}

func modInvEGCD(e *big.Int, phi *big.Int) *big.Int {
    _, x, _ := extGCD(e, phi)
    // ensure positive
    x.Mod(x, phi)
    if x.Cmp(big.NewInt(0)) < 0 {
        x.Add(x, phi)
    }
    return x
}


// func main(){
// 	test := big.NewInt(65537)
// 	fmt.Printf("test: %s\n", test.Text(16))
// }
// 
 // func main(){
 // 	a := big.NewInt(17);
 // 	b := big.NewInt(19);
 // 	m := big.NewInt(23);
 // 	fmt.Printf("Montgomery form is: %d\n", montgomeryMult(a,b,m));
// 	p := generatePrime(128, 20)
// 	q := generatePrime(128, 20)
// 	pubExp := big.NewInt(65537)
// 	pMinus1 := new(big.Int).Sub(p, big.NewInt(1))
// 	qMinus1 := new(big.Int).Sub(q, big.NewInt(1))
// 	totient := new(big.Int).Mul(pMinus1, qMinus1)
// 	n := new(big.Int).Mul(p, q)
// 	d := modInvEGCD(pubExp, totient)
// 	
// 	msg := "hello"
// 	m := new(big.Int).SetBytes([]byte(msg))
// 	fmt.Printf("m < n: %v\n", m.Cmp(n) < 0)
// 	
// 	c := montLadder(m, pubExp, n)
// 	decrypted := montLadder(c, d, n)
// 	fmt.Printf("Original:  %s\n", msg)
// 	fmt.Printf("Decrypted: %s\n", string(decrypted.Bytes()))
// 	fmt.Printf("Match: %v\n", m.Cmp(decrypted) == 0)
//}
//  func main(){
// // 
// // 	encrypted := simpleXORCipher("DH_VERIFIED+password", "jksklahoijwoahgfGHHUIGIKUIBHUI");
// // 	fmt.Printf("Encryption: %v\n", []byte(encrypted));
// // 	fmt.Printf("Encrypted (hex) : %s\n", hex.EncodeToString([]byte(encrypted)));
// // // 	bitLength := 16;
// // // 	fmt.Println("=================================");
// // // 
// // // 	candidate := generateCandidatePrime(bitLength);
// // // 	s, d := prepCandidate(candidate);
// // // 
// // // 	fmt.Printf("Candidate:	%d\n", candidate);
// // // 
// // // 	fmt.Printf("Miller-Rabin: %t\n", MillerRabin(s,d, 20));
// // // 	fmt.Println("==================================");
// // // 
// 	// g := big.NewInt(5);
// 	// k := big.NewInt(11);
// 	// n := big.NewInt(7);
//  // 	fmt.Printf("Mont ladder 5^11 mod 7: %d (expected 3)\n", montLadder(g,k,n));
// 	e := big.NewInt(5);
// 	totient := big.NewInt(11);
//  	fmt.Printf("Inv 5^{-1} mod 11: %d (expected 9)\n", modInvEGCD(e, totient));
// 
// // // 
// // // 	fmt.Printf("Prime:		%d\n", generatePrime(bitLength, 20));
// }


// 	for R <= modulus  {
// 		R <<= 1;
// 	}
// 	var n_inv big.Int = modinv(modulus , R);
// 	var n_prime big.Int = (R - n_inv) & (R - 1);
// 	
// 	var m big.Int = (T * n_prime) & (R - 1);
// 	var t big.Int = (T + m * modulus) / R;
// 
// 	if t >= modulus {
// 		t -= modulus;
// 	}
// 	//fmt.Printf("t is : %d\n", t);
// 	return t;
// }


// 	for R <= modulus  {
// 		R <<= 1;
// 	}
// 	var a_mont big.Int = (a*R) % modulus;
// 	var b_mont big.Int = (b*R) % modulus;
// 
// 	//fmt.Printf("a,b,modulus, R, n_prime: %d %d %d %d %d\n", a, b, modulus, R, n_prime);
// 	return montgomeryRedc(a_mont*b_mont, modulus);
// }

// func montLadder(g big.Int, k big.Int, n big.Int) big.Int{
// 	var r_0 big.Int = montgomeryMult(1,1,n);
// 	var r_1 big.Int = montgomeryMult(g,1,n);
// 	binK := toBinary(k);
// 	slices.Reverse(binK[:]);
// 	for _, bit := range binK	{
// 		if bit == 0 {
// 			r_1 = montgomeryRedc(r_1*r_0, n); // (r_1 * r_0) % n;
// 			r_0 = montgomeryRedc(r_0*r_0, n); //(r_0 * r_0) % n;
// 		} else{
// 			r_0 = montgomeryRedc(r_1*r_0, n); //(r_1 * r_0) % n;
// 			r_1 = montgomeryRedc(r_1*r_1, n); //(r_1 * r_1) % n;
// 		}
// 	}
// 	return montgomeryRedc(r_0, n);
// }

// func MillerRabin(s *big.Int, d *big.Int, rounds int) bool {
// 	// n = (2^s * d) + 1. Therefore n - 1 = 2^s * d
// 	nMinusOne := (big.Int(1) << *s) * (*d)
// 	n := nMinusOne + 1
// 
// 	for i := 0; i < rounds; i++ {
// 		a := big.Int(rand.Int63n(int64(n-3))) + 2;
// 		
// 		x := montLadder(a, *d, n);
// 
// 		if x == 1 || x == nMinusOne {
// 			continue
// 		}
// 
// 		composite := true
// 		for r := big.Int(1); r < *s; r++ {
// 		 
// 			x = montLadder(x, 2, n);// (x * x) % n 
// 
// 			if x == nMinusOne {
// 				composite = false;
// 				break
// 			}
// 		}
// 
// 		if composite {
// 			return false
// 		}
// 	}
// 
// 	return true // Probably prime
// }

// Just use 2 as the generator
// func generateGenerator(modulus big.Int) big.Int{
// 	return big.Int(rand.Int63n(modulus));
// }




// func main(){
// 	var bitLength int = 5;
// 	largePrime := generateCandidatePrime(&bitLength);
// 	s, d := prepCandidate(largePrime);
// 	fmt.Printf("------------------------------\n");
// 	fmt.Printf("The large Prime is: %d\n", largePrime);
// 	fmt.Printf("------------------------------\n");
// 	fmt.Printf("Candidate Prep: %d \t %d\n", s, d);
// 	fmt.Printf("Verification: 2^%d * %d = %d\n", s, d, (1<<s)*d);
// 	fmt.Printf("------------------------------\n");
// 	// var num big.Int = 25;
// 	// bits := toBinary(num);
// 	// length := len(bits);
// 	// for i:=0; i < length; i++{
// 	// 	fmt.Printf("To binary is: %d\n",bits[i]);	
// 	// }
// 	fmt.Printf("Miller Rabin : %t\n", MillerRabin(&s, &d, 20));
// 	fmt.Printf("------------------------------\n");
// 	var g, k, n big.Int = 5, 11, 7;
// 	fmt.Printf("Mont ladder: %d\n", montLadder(g, k, n));
// 	// var result big.Int = montgomeryMult(5,3,11);
// 	// fmt.Printf("Montgomery multiplication : %d\n", montgomeryRedc(result, 11));
// }
