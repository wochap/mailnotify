package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DusanKasan/parsemail"
	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
	"gopkg.in/fsnotify.v1"
)

var htmlEscaper = strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;")

func main() {
	log.SetFlags(0)

	// Connect to DBus for sending the notification
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err = conn.Auth(nil); err != nil {
		log.Fatal(err)
	}

	if err = conn.Hello(); err != nil {
		log.Fatal(err)
	}

	// Watch the filesystem
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	iconPath := ""
	if len(os.Args) > 2 {
		iconPath = os.Args[2]
	}

	done := make(chan bool)
	// Wait for files to be created, then parse them and show the basic info.
	go func() {
		for {
			event, ok := <-watcher.Events
			if !ok {
				return
			}
			if event.Op&fsnotify.Create != fsnotify.Create {
				continue
			}

			log.Printf("New file: %s", event.Name)

			file, err := os.Open(event.Name)
			if err != nil {
				log.Println("error opening file: ", err)
				continue
			}
			defer file.Close()

			email, err := parsemail.Parse(file)
			if err != nil {
				log.Println("error parsing mail file: ", err)
				continue
			}

			body := fmt.Sprintf(`<i>From: %s</i><br><i>To: %s</i><br><br>%s`,
				htmlEscaper.Replace(email.From[0].String()),
				htmlEscaper.Replace(email.Header.Get("Delivered-To")),
				htmlEscaper.Replace(email.TextBody[:100]))

			n := notify.Notification{
				AppName:       "Mail",
				ReplacesID:    uint32(0),
				AppIcon:       iconPath,
				Summary:       email.Subject,
				Body:          body,
				Hints:         map[string]dbus.Variant{},
				ExpireTimeout: time.Second * -1,
			}

			createdID, err := notify.SendNotification(conn, n)
			if err != nil {
				log.Printf("error sending notification: %v", err.Error())
			}
			log.Printf("created notification with id: %v", createdID)
		}
	}()

	err = filepath.Walk(os.Args[1], func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "new" && filepath.Base(filepath.Dir(path)) == "INBOX" {
			log.Printf("Watching %s", path)
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
