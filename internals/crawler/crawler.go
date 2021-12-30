package crawler

import (
	"bufio"
	"compress/gzip"
	"crypto/tls"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

import _ "net/http/pprof"

type Domain struct {
	Name     string
	Reg      string
	Create   string
	PaidTill string
	FreeDate string
	TLSVer   string
}

// TODO: rewrite this sheet to reflect
func (d Domain) Write() []string {
	s := []string{d.Name, d.TLSVer}
	return s
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func Run(limit int) {
	//flag.Parse()
	//if *cpuprofile != "" {
	//	f, err := os.Create(*cpuprofile)
	//	if err != nil {
	//		log.Fatal("could not create CPU profile: ", err)
	//	}
	//	defer f.Close() // error handling omitted for example
	//	if err := pprof.StartCPUProfile(f); err != nil {
	//		log.Fatal("could not start CPU profile: ", err)
	//	}
	//	defer pprof.StopCPUProfile()
	//}
	//
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()

	var (
		zones [1]string
		err   error
	)
	const (
		workers = 1000
	)
	zones[0] = "ru"

	zoneUrl := fmt.Sprintf("https://partner.r01.ru/zones/%s_domains.gz", zones[0])
	zoneFile := fmt.Sprintf("%s_zone.gz", zones[0])
	zoneFileDecompressed := fmt.Sprintf("%s_zone", zones[0])

	if CacheExistsAndValid(zoneFileDecompressed) {
		log.Println("Skipping download")
		Domains := LoadData(zoneFileDecompressed, limit)
		CheckTLSVersion(Domains, workers)
	}
	err = DownloadFile(zoneFile, zoneUrl)
	if err != nil {
		log.Fatal(err)
	}
	err = DecompressFile(zoneFile, zoneFileDecompressed)
	if err != nil {
		log.Fatal(err)
	}

	Domains := LoadData(zoneFileDecompressed, limit)
	CheckTLSVersion(Domains, workers)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}

func DownloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	buf := make([]byte, 1024)
	bytesRead := 0

	if err != nil {
		return err
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				_, err = out.Write(buf[:n])
				if err != nil {
					log.Fatal(err)
				}
				break
			}
			log.Fatal(err)
		}
		_, err = out.Write(buf[:n])
		if err != nil {
			log.Fatal(err)
		}
		bytesRead += n
	}
	defer resp.Body.Close()
	return err
}

func DecompressFile(gzipped string, ungzipped string) error {
	buf := make([]byte, 1024)
	input, err := os.Open(gzipped)
	if err != nil {
		log.Fatal(err)
	}
	r, err := gzip.NewReader(input)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := os.Create(ungzipped)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				_, err = out.Write(buf[:n])
				if err != nil {
					log.Fatal(err)
				}
				break
			}
			log.Fatal(err)
		}

		fmt.Println(n)
		_, err = out.Write(buf[:n])
		if err != nil {
			log.Fatal(err)
		}

	}
	return err
}
func CacheExistsAndValid(filepath string) bool {
	var ttl int64
	ttl = 86400
	FileInfo, err := os.Stat(filepath)
	CacheValid := true
	if err != nil {
		log.Println("no cache file found")
		CacheValid = false
		return CacheValid
	}
	FileCacheTime := FileInfo.ModTime().Unix()
	if time.Now().Unix()-FileCacheTime > ttl {
		log.Printf("Cache expired, downloading new file")
		CacheValid = false
	}
	return CacheValid
}

func LoadData(dataFilePath string, limit int) []Domain {
	var Domains []Domain
	fd, err := os.Open(dataFilePath)
	if err != nil {
		log.Fatal(err)
	}

	s := bufio.NewScanner(fd)
	s.Split(bufio.ScanLines)
	count := 0
	for s.Scan() {
		if limit == -1 || count < limit {
			line := strings.Fields(s.Text())
			Domains = append(Domains, Domain{line[0], line[1], line[2], line[3], line[4], ""})
			count++
		}
	}
	return Domains
}

func CheckTLSVersion(Domains []Domain, workers int) {
	var (
		wg  sync.WaitGroup
		ch  = make(chan Domain)
		ch2 = make(chan Domain)
	)
	// create threads waiting on channel
	for i := 1; i < workers; i++ {
		wg.Add(1)
		go WrapSendRequest(ch, ch2, &wg)
	}

	// create writer to csv
	wg.Add(1)
	fmt.Println("qqq")
	go WriteCSVFile(ch2, &wg)

	// Generate data and send to channel
	for i := 0; i < len(Domains); i++ {
		fmt.Println("read from ch")
		ch <- Domains[i]
	}
	close(ch)

	// Close the channel when all goroutines are finished
	//go func() {
	//	wg.Wait()
	//	close(ch2)
	//}()
	wg.Wait()
}

func WrapSendRequest(ch chan Domain, ch2 chan Domain, wg *sync.WaitGroup) {
	defer wg.Done()
	// read data from channel and do staff
	for domain := range ch {
		fmt.Println("got")
		domain = SendRequest(domain)
		ch2 <- domain
	}
	fmt.Println("Worker finished work")
}
func WriteCSVFile(ch chan Domain, wg *sync.WaitGroup) {
	fmt.Println("asda")

	defer wg.Done()

	fd, _ := os.Create("./out.csv")
	w := csv.NewWriter(fd)
	for domain := range ch {
		w.Write(domain.Write())
		fmt.Printf("TLSVer: Domain: %s %s TLSVerEnd\n", domain.Name, domain.TLSVer)
	}
	w.Flush()
	fmt.Println("done write")
	// when no data in the channel finish goroutine
	//close(ch)
}
func SendRequest(domain Domain) Domain {
	var (
		tlsVersion string
	)
	url := fmt.Sprintf("https://%s", domain.Name)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(url)
	if err != nil {
		//log.Println(err)
		domain.TLSVer = string(err.Error())
		return domain
	}
	if resp.StatusCode == 200 {
		log.Printf("domain: %s status code: %d", domain.Name, resp.StatusCode)
		if resp.TLS.Version == tls.VersionTLS10 {
			tlsVersion = "TLS 1.0"
		} else if resp.TLS.Version == tls.VersionTLS11 {
			tlsVersion = "TLS 1.1"
		} else if resp.TLS.Version == tls.VersionTLS12 {
			tlsVersion = "TLS 1.2"
		} else if resp.TLS.Version == tls.VersionTLS13 {
			tlsVersion = "TLS 1.3"
		} else if resp.TLS.Version == tls.VersionSSL30 {
			tlsVersion = "SSLv3"
		} else {
			tlsVersion = "unknown"
		}
		domain.TLSVer = tlsVersion
	}
	return domain
}
