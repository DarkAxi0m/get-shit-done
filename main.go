package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var (
	startToken = "## start-gsd"
	endToken   = "## end=gsd"
	hostsFile  = "/etc/hosts"
)

func main() {
	sudoUser := os.Getenv("SUDO_USER")
	userHome := os.Getenv("HOME")
	if sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil {
			userHome = u.HomeDir
		}
	}

	iniPath := flag.String("config", filepath.Join(userHome, ".config", "get-shit-done.ini"), "INI file with additional sites")
	dryRun := flag.Bool("dry-run", false, "Show changes but don't modify files")
	flag.Parse()

	usr, err := user.Current()
	if err != nil || usr.Uid != "0" {
		exitWithError("Please run as root", 2)
	}

	ensureIniFile(*iniPath)

	if flag.NArg() == 0 {
		exitWithError("Missing mode. Use: work, play, add, remove, list, or status", 1)
	}

	action := flag.Arg(0)
	arg := ""
	if flag.NArg() > 1 {
		arg = flag.Arg(1)
	}

	switch action {
	case "work":
		if err := work(*iniPath, *dryRun); err != nil {
			exitWithError(err.Error(), 1)
		}
		notify("Work mode activated")
	case "play":
		if err := play(*dryRun); err != nil {
			exitWithError(err.Error(), 1)
		}
		notify("Play mode activated")
	case "add":
		if arg == "" {
			exitWithError("Please provide a domain to add", 1)
		}
		if err := modifyIniDomain(*iniPath, arg, true); err != nil {
			exitWithError(err.Error(), 1)
		}
		notify("Domain added: " + arg)
	case "remove":
		if arg == "" {
			exitWithError("Please provide a domain to remove", 1)
		}
		if err := modifyIniDomain(*iniPath, arg, false); err != nil {
			exitWithError(err.Error(), 1)
		}
		notify("Domain removed: " + arg)
	case "list":
		domains, err := domainsFromIni(*iniPath)
		if err != nil {
			exitWithError("Unable to read config: "+err.Error(), 1)
		}
		fmt.Println("Blocked domains:")
		for _, d := range domains {
			fmt.Println(" -", strings.TrimSpace(d))
		}
	case "status":
		printStatus()
	default:
		exitWithError("Invalid mode. Use 'work', 'play', 'add', 'remove', 'list', or 'status'", 5)
	}
}

func ensureIniFile(path string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("sites=facebook.com,\n"), 0o644)
}

func modifyIniDomain(path, domain string, add bool) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil
	}

	existing, _ := domainsFromIni(path)
	set := make(map[string]struct{})
	for _, d := range existing {
		set[strings.TrimSpace(d)] = struct{}{}
	}

	if add {
		set[domain] = struct{}{}
	} else {
		delete(set, domain)
	}

	combined := make([]string, 0, len(set))
	for d := range set {
		combined = append(combined, d)
	}
	sort.Strings(combined)
	entry := "sites=" + strings.Join(combined, ",") + ","

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(entry + "\n")
	return err
}

func domainsFromIni(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "sites=") {
			cleaned := strings.Trim(line[6:], ",")
			domains = append(domains, strings.Split(cleaned, ",")...)
		}
	}
	return domains, scanner.Err()
}

func work(iniFile string, dryRun bool) error {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return errors.New("No hosts file found")
	}

	if strings.Contains(string(content), startToken) && strings.Contains(string(content), endToken) {
		return errors.New("Work mode already set")
	}

	extra, _ := domainsFromIni(iniFile)
	domainSet := map[string]struct{}{}
	for _, d := range extra {
		clean := strings.ToLower(strings.TrimSpace(d))
		if clean != "" {
			domainSet[clean] = struct{}{}
		}
	}

	sorted := make([]string, 0, len(domainSet))
	for d := range domainSet {
		sorted = append(sorted, d)
	}
	sort.Strings(sorted)

	if dryRun {
		fmt.Println("Dry run: Would add the following entries to hosts file:")
		fmt.Println(startToken)
		for _, domain := range sorted {
			fmt.Printf("127.0.0.1\t%s\n127.0.0.1\twww.%s\n", domain, domain)
		}
		fmt.Println(endToken)
		return nil
	}

	_ = os.WriteFile(hostsFile+".bak", content, 0o644)
	file, err := os.OpenFile(hostsFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, startToken); err != nil {
		return err
	}
	for _, domain := range sorted {
		fmt.Fprintf(file, "127.0.0.1\t%s\n127.0.0.1\twww.%s\n", domain, domain)
	}
	if _, err := fmt.Fprintln(file, endToken); err != nil {
		return err
	}

	return restartNetwork()
}

func play(dryRun bool) error {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return errors.New("No hosts file found")
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	inBlock := false
	var result []string
	var removed []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, startToken) {
			inBlock = true
			removed = append(removed, line)
			continue
		}
		if strings.Contains(line, endToken) {
			inBlock = false
			removed = append(removed, line)
			continue
		}
		if inBlock {
			removed = append(removed, line)
			continue
		}
		result = append(result, line)
	}

	if dryRun {
		fmt.Println("Dry run: Would remove the following lines from hosts file:")
		for _, line := range removed {
			fmt.Println(line)
		}
		return nil
	}

	_ = os.WriteFile(hostsFile+".bak", content, 0o644)
	return os.WriteFile(hostsFile, []byte(strings.Join(result, "\n")+"\n"), 0o644)
}

func printStatus() {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		fmt.Println("Could not read hosts file")
		return
	}
	if strings.Contains(string(content), startToken) && strings.Contains(string(content), endToken) {
		fmt.Println("Current mode: WORK")
	} else {
		fmt.Println("Current mode: PLAY")
	}
}

func restartNetwork() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("/etc/init.d/networking", "restart")
	case "darwin":
		cmd = exec.Command("dscacheutil", "-flushcache")
	default:
		fmt.Println("Please contribute DNS cache flush command on GitHub")
		return nil
	}
	return cmd.Run()
}

func notify(msg string) {
	exec.Command("notify-send", "Get Shit Done", msg).Run()
}

func exitWithError(msg string, code int) {
	fmt.Fprintln(os.Stderr, msg)
	printHelp()
	os.Exit(code)
}

func printHelp() {
	fmt.Println("Usage: get-shit-done [work|play|add|remove] [-config=path] [--dry-run] [--status]")
}
