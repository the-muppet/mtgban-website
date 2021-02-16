package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/kodabb/go-mtgban/mtgmatcher"

	"github.com/mackerelio/go-osstat/memory"
	"golang.org/x/sys/unix"
)

const (
	mtgjsonURL = "https://mtgjson.com/api/v5/AllPrintings.json"
)

func Admin(w http.ResponseWriter, r *http.Request) {
	sig := getSignatureFromCookies(r)

	pageVars := genPageNav("Admin", sig)

	msg := r.FormValue("msg")
	if msg != "" {
		pageVars.InfoMessage = msg
	}

	refresh := r.FormValue("refresh")
	if refresh != "" {
		key, found := ScraperMap[refresh]
		if !found {
			pageVars.InfoMessage = refresh + " not found"
		}
		if key != "" {
			_, found := ScraperOptions[key]
			if !found {
				pageVars.InfoMessage = key + " not found"
			} else {
				// Strip the request parameter to avoid accidental repeats
				// and to give a chance to table to update
				r.URL.RawQuery = ""
				if ScraperOptions[key].Busy {
					v := url.Values{
						"msg": {key + " is already being refreshed"},
					}
					r.URL.RawQuery = v.Encode()
				} else if len(ScraperOptions[key].Keepers) > 0 {
					go reloadMarket(key)
				} else {
					go reloadSingle(key)
				}

				http.Redirect(w, r, r.URL.String(), http.StatusFound)
				return
			}
		}
	}
	reboot := r.FormValue("reboot")
	doReboot := false
	var v url.Values
	switch reboot {
	case "mtgstocks":
		v = url.Values{}
		v.Set("msg", "Refreshing MTGStocks in the background...")
		doReboot = true
		go loadInfos()

	case "mtgjson":
		v = url.Values{}
		v.Set("msg", "Reloading MTGJSON in the background...")
		doReboot = true

		go func() {
			log.Println("Retrieving the latest version of mtgjson")
			resp, err := cleanhttp.DefaultClient().Get(mtgjsonURL)
			if err != nil {
				log.Println(err)
				return
			}
			defer resp.Body.Close()

			log.Println("Installing the new mtgjson version")
			err = mtgmatcher.LoadDatastore(resp.Body)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("New mtgjson is ready")
		}()

	case "scrapers":
		v = url.Values{}
		v.Set("msg", "Reloading scrapers in the background...")
		doReboot = true

		skip := false
		for key, opt := range ScraperOptions {
			if opt.Busy {
				v.Set("msg", "Cannot reload everything while "+key+" is refreshing")
				skip = true
				break
			}
		}

		if !skip {
			go func() {
				loadScrapers(true, true)
			}()
		}

	case "server":
		v = url.Values{}
		v.Set("msg", "Restarting the server...")
		doReboot = true

		// Let the system restart the server
		go func() {
			time.Sleep(5 * time.Second)
			log.Println("Admin requested server restart")
			os.Exit(0)
		}()
	}
	if doReboot {
		r.URL.RawQuery = v.Encode()
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
		return
	}

	pageVars.Headers = []string{
		"Name", "Short", "Last Update", "Entries", "Status",
	}
	for i := range Sellers {
		if Sellers[i] == nil {
			row := []string{
				fmt.Sprintf("Error at Seller %d", i), "", "", "", "",
			}
			pageVars.Table = append(pageVars.Table, row)
			continue
		}

		scraperOptions := ScraperOptions[ScraperMap[Sellers[i].Info().Shorthand]]

		lastUpdate := Sellers[i].Info().InventoryTimestamp.Format(time.Stamp)

		inv, _ := Sellers[i].Inventory()

		status := "✅"
		if scraperOptions.Busy {
			status = "🔶"
		} else if len(inv) == 0 {
			status = "🔴"
		}

		row := []string{
			Sellers[i].Info().Name,
			Sellers[i].Info().Shorthand,
			lastUpdate,
			fmt.Sprint(len(inv)),
			status,
		}

		pageVars.Table = append(pageVars.Table, row)
	}

	for i := range Vendors {
		if Vendors[i] == nil {
			row := []string{
				fmt.Sprintf("Error at Vendor %d", i), "", "", "", "",
			}
			pageVars.Table = append(pageVars.Table, row)
			continue
		}

		scraperOptions := ScraperOptions[ScraperMap[Vendors[i].Info().Shorthand]]

		lastUpdate := Vendors[i].Info().BuylistTimestamp.Format(time.Stamp)

		bl, _ := Vendors[i].Buylist()

		status := "✅"
		if scraperOptions.Busy {
			status = "🔶"
		} else if len(bl) == 0 {
			status = "🔴"
		}

		row := []string{
			Vendors[i].Info().Name,
			Vendors[i].Info().Shorthand,
			lastUpdate,
			fmt.Sprint(len(bl)),
			status,
		}

		pageVars.OtherTable = append(pageVars.OtherTable, row)
	}

	pageVars.Uptime = uptime()
	pageVars.DiskStatus = disk()
	pageVars.MemoryStatus = memory()
	pageVars.CurrentTime = time.Now()

	render(w, "admin.html", pageVars)
}

// Custom time.Duration format to print days as well
func uptime() string {
	since := time.Now().Sub(startTime)
	days := int(since.Hours() / 24)
	hours := int(since.Hours()) % 24
	minutes := int(since.Minutes()) % 60
	seconds := int(since.Seconds()) % 60
	return fmt.Sprintf("%d days, %02d:%02d:%02d", days, hours, minutes, seconds)
}

func disk() string {
	wd, err := os.Getwd()
	if err != nil {
		return "N/A"
	}
	var stat unix.Statfs_t
	unix.Statfs(wd, &stat)

	total := stat.Blocks * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	used := total - avail

	return fmt.Sprintf("%.2f%% of %.2fGB", float64(used)/float64(total)*100, float64(total)/1024/1024/1024)
}

func mem() string {
	memData, err := memory.Get()
	if err != nil {
		return "N/A"
	}
	return fmt.Sprintf("%.2f%% of %.2fGB", float64(memData.Used)/float64(memData.Total)*100, float64(memData.Total)/1024/1024/1024)
}
