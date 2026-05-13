package main

import (
	"fmt"
	"math/rand"
	"slices"
)

func fromBinary(bits []int) uint64{
	var result uint64 = 0;
	length_bits := len(bits);
	for i:=0; i < length_bits; i++{
		if bits[i] == 1 {
			result |= (1 << i);
		}
		//fmt.Printf("The result is: %d\n", result);
	}
	
	return uint64(result);
}


func generateCandidatePrime(bitlen *int) uint64{
	length := *bitlen;
	
	bits := []int{1};

	for i:= 0; i < (length - 2); i++ {
		bits = append(bits, rand.Intn(2));	
	}
	
	bits = append(bits, 1);
	
	return fromBinary(bits);
}

func prepCandidate(candidate uint64) (uint64, uint64){
	var s uint64 = 0;
	var d uint64 = candidate - 1;
	for (d % 2) == 0 {
		d >>= 1;
		s += 1;
	}
	return s, d;
}

func toBinary(num uint64) []uint64 {
	var bits []uint64;
	if num == 0 {
		return []uint64{0};
	}
	for num > 0 {
		bits = append(bits, num%2);
		num = num >> 1;
	}

	return bits;
}

func modinv(n uint64, R uint64) uint64 {
	var x uint64 = 1;
	for i:=0; i<6; i++ {
		x = x * (2 - n * x);
	}	
	return x & (R - 1);
}

func montgomeryRedc(T uint64, modulus uint64) uint64 {
	var R uint64 = 1;
	for R <= modulus  {
		R <<= 1;
	}
	var n_inv uint64 = modinv(modulus , R);
	var n_prime uint64 = (R - n_inv) & (R - 1);
	
	var m uint64 = (T * n_prime) & (R - 1);
	var t uint64 = (T + m * modulus) / R;

	if t >= modulus {
		t -= modulus;
	}
	//fmt.Printf("t is : %d\n", t);
	return t;
}

func montgomeryMult(a uint64, b uint64, modulus uint64) uint64 {

	var R uint64 = 1;
	for R <= modulus  {
		R <<= 1;
	}
	var a_mont uint64 = (a*R) % modulus;
	var b_mont uint64 = (b*R) % modulus;

	//fmt.Printf("a,b,modulus, R, n_prime: %d %d %d %d %d\n", a, b, modulus, R, n_prime);
	return montgomeryRedc(a_mont*b_mont, modulus);
}


func montLadder(g uint64, k uint64, n uint64) uint64{
	var r_0 uint64 = montgomeryMult(1,1,n);
	var r_1 uint64 = montgomeryMult(g,1,n);
	binK := toBinary(k);
	slices.Reverse(binK[:]);
	for _, bit := range binK	{
		if bit == 0 {
			r_1 = montgomeryRedc(r_1*r_0, n); // (r_1 * r_0) % n;
			r_0 = montgomeryRedc(r_0*r_0, n); //(r_0 * r_0) % n;
		} else{
			r_0 = montgomeryRedc(r_1*r_0, n); //(r_1 * r_0) % n;
			r_1 = montgomeryRedc(r_1*r_1, n); //(r_1 * r_1) % n;
		}
	}
	return montgomeryRedc(r_0, n);
}

func MillerRabin(s *uint64, d *uint64, rounds int) bool {
	// n = (2^s * d) + 1. Therefore n - 1 = 2^s * d
	nMinusOne := (uint64(1) << *s) * (*d)
	n := nMinusOne + 1

	for i := 0; i < rounds; i++ {
		a := uint64(rand.Int63n(int64(n-3))) + 2;
		
		x := montLadder(a, *d, n);

		if x == 1 || x == nMinusOne {
			continue
		}

		composite := true
		for r := uint64(1); r < *s; r++ {
			// We need x = x^2 % n. 
			x = montLadder(x, 2, n);// (x * x) % n 

			if x == nMinusOne {
				composite = false;
				break
			}
		}

		if composite {
			return false
		}
	}

	return true // Probably prime
}


func DiffieHellman(rounds int){
	var genModulus [2]uint64; // [modulus, generator]
	var bitLength int = 256;
	for {
		candidatePrime := generateCandidatePrime(&bitLength);
		s, d := prepCandidate(candidatePrime);
		ok := MillerRabin(&s, &d, rounds);
		if ok{
			possiblePrime := ((1 << s) * d) + 1;
			genModulus = append(genModulus, possiblePrime);
			break; 
		}
	}
	genModulus = append(genModulus, uint64(rand.Int63n(genModulus[0])));

	return 0;
}

func main(){
	var bitLength int = 5;
	largePrime := generateCandidatePrime(&bitLength);
	s, d := prepCandidate(largePrime);
	fmt.Printf("------------------------------\n");
	fmt.Printf("The large Prime is: %d\n", largePrime);
	fmt.Printf("------------------------------\n");
	fmt.Printf("Candidate Prep: %d \t %d\n", s, d);
	fmt.Printf("Verification: 2^%d * %d = %d\n", s, d, (1<<s)*d);
	fmt.Printf("------------------------------\n");
	// var num uint64 = 25;
	// bits := toBinary(num);
	// length := len(bits);
	// for i:=0; i < length; i++{
	// 	fmt.Printf("To binary is: %d\n",bits[i]);	
	// }
	fmt.Printf("Miller Rabin : %t\n", MillerRabin(&s, &d, 20));
	fmt.Printf("------------------------------\n");
	var g, k, n uint64 = 5, 11, 7;
	fmt.Printf("Mont ladder: %d\n", montLadder(g, k, n));
	// var result uint64 = montgomeryMult(5,3,11);
	// fmt.Printf("Montgomery multiplication : %d\n", montgomeryRedc(result, 11));
}
