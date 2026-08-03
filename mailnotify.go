package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DusanKasan/parsemail"
	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
	"gopkg.in/fsnotify.v1"
)

var htmlEscaper = strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;")

// shouldWatch reports whether path is a mail delivery directory. mbsync
// delivers to <account>/INBOX/new; lieer writes new messages to
// <account>/mail/tmp and renames them into <account>/mail/cur (unread state
// is a maildir flag, flag changes rename files within cur).
func shouldWatch(path string, info os.FileInfo) bool {
	if !info.IsDir() {
		return false
	}
	switch filepath.Base(filepath.Dir(path)) {
	case "INBOX":
		return info.Name() == "new"
	case "mail":
		return info.Name() == "new" || info.Name() == "cur" || info.Name() == "tmp"
	}
	return false
}

type hintFlags []string

func (h *hintFlags) String() string { return strings.Join(*h, " ") }

func (h *hintFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

// iconFlag lets --icon and --app-icon share a destination so that the last
// one on the command line wins, like notify-send.
type iconFlag struct {
	path *string
}

func (f iconFlag) String() string {
	if f.path == nil {
		return ""
	}
	return *f.path
}

func (f iconFlag) Set(value string) error {
	*f.path = value
	return nil
}

// parseUrgency maps a notify-send urgency level to its spec value.
func parseUrgency(level string) (byte, error) {
	switch level {
	case "low":
		return 0, nil
	case "normal":
		return 1, nil
	case "critical":
		return 2, nil
	}
	return 0, fmt.Errorf("invalid urgency %q: expected low, normal, or critical", level)
}

// parseHint parses a notify-send-style hint spec (type:name:value).
func parseHint(spec string) (string, dbus.Variant, error) {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) != 3 {
		return "", dbus.Variant{}, fmt.Errorf("invalid hint %q: expected type:name:value", spec)
	}
	name, typ, value := parts[1], parts[0], parts[2]

	switch typ {
	case "boolean":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return "", dbus.Variant{}, fmt.Errorf("invalid boolean hint %q: %w", spec, err)
		}
		return name, dbus.MakeVariant(v), nil
	case "int":
		v, err := strconv.Atoi(value)
		if err != nil {
			return "", dbus.Variant{}, fmt.Errorf("invalid int hint %q: %w", spec, err)
		}
		return name, dbus.MakeVariant(int32(v)), nil
	case "double":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", dbus.Variant{}, fmt.Errorf("invalid double hint %q: %w", spec, err)
		}
		return name, dbus.MakeVariant(v), nil
	case "string":
		return name, dbus.MakeVariant(value), nil
	case "byte":
		v, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return "", dbus.Variant{}, fmt.Errorf("invalid byte hint %q: %w", spec, err)
		}
		return name, dbus.MakeVariant(byte(v)), nil
	case "variant":
		return name, dbus.MakeVariant(dbus.MakeVariant(value)), nil
	}
	return "", dbus.Variant{}, fmt.Errorf("invalid hint type %q in %q: expected boolean, int, double, string, byte, or variant", typ, spec)
}

func main() {
	log.SetFlags(0)

	var (
		appName    string
		urgency    string
		expireTime int
		iconPath   string
		hints      hintFlags
	)
	icon := iconFlag{path: &iconPath}
	flag.StringVar(&appName, "a", "Mail", "app name for the notification")
	flag.StringVar(&appName, "app-name", "Mail", "app name for the notification")
	flag.StringVar(&urgency, "u", "", "urgency level: low, normal, or critical")
	flag.StringVar(&urgency, "urgency", "", "urgency level: low, normal, or critical")
	flag.IntVar(&expireTime, "t", -1, "expire time in milliseconds (-1 lets the notification server decide, 0 never expires)")
	flag.IntVar(&expireTime, "expire-time", -1, "expire time in milliseconds (-1 lets the notification server decide, 0 never expires)")
	flag.Var(icon, "i", "icon filename or stock icon to display")
	flag.Var(icon, "icon", "icon filename or stock icon to display")
	flag.Var(icon, "n", "application icon filename or app icon name")
	flag.Var(icon, "app-icon", "application icon filename or app icon name")
	flag.Var(&hints, "h", "extra notification hint (TYPE:NAME:VALUE), types: boolean, int, double, string, byte, variant; repeatable")
	flag.Var(&hints, "hint", "extra notification hint (TYPE:NAME:VALUE), types: boolean, int, double, string, byte, variant; repeatable")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [flags] MAILDIR\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	mailDir := args[0]

	hintMap := map[string]dbus.Variant{}
	if urgency != "" {
		level, err := parseUrgency(urgency)
		if err != nil {
			log.Fatal(err)
		}
		hintMap["urgency"] = dbus.MakeVariant(level)
	}
	for _, h := range hints {
		name, variant, err := parseHint(h)
		if err != nil {
			log.Fatal(err)
		}
		hintMap[name] = variant
	}

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

	// pending tracks message basenames created in a maildir tmp directory.
	// lieer writes new messages to mail/tmp and renames them into mail/cur,
	// while read/star changes rename files within mail/cur; only notify for
	// tmp->cur arrivals so flag renames are ignored.
	pending := map[string]bool{}

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

			switch filepath.Base(filepath.Dir(event.Name)) {
			case "tmp":
				pending[filepath.Base(event.Name)] = true
				continue
			case "cur":
				base := filepath.Base(event.Name)
				if !pending[base] {
					continue
				}
				delete(pending, base)
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

			from := ""
			if len(email.From) > 0 {
				from = email.From[0].String()
			}

			preview := email.TextBody
			if len(preview) > 100 {
				preview = preview[:100]
			}

			body := fmt.Sprintf(`<i>From: %s</i><br><i>To: %s</i><br><br>%s`,
				htmlEscaper.Replace(from),
				htmlEscaper.Replace(email.Header.Get("Delivered-To")),
				htmlEscaper.Replace(preview))

			n := notify.Notification{
				AppName:       appName,
				ReplacesID:    uint32(0),
				AppIcon:       iconPath,
				Summary:       email.Subject,
				Body:          body,
				Hints:         hintMap,
				ExpireTimeout: time.Duration(expireTime) * time.Millisecond,
			}

			createdID, err := notify.SendNotification(conn, n)
			if err != nil {
				log.Printf("error sending notification: %v", err.Error())
			}
			log.Printf("created notification with id: %v", createdID)
		}
	}()

	err = filepath.Walk(mailDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Base(filepath.Dir(path)) == "tmp" {
			// resume an interrupted sync: tmp files already on disk will be
			// renamed into cur without a fresh tmp create event
			pending[info.Name()] = true
			return nil
		}
		if shouldWatch(path, info) {
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
