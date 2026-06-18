package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/imrx44/redbrut/internal/input"
	"github.com/imrx44/redbrut/internal/output"
	"github.com/imrx44/redbrut/internal/stats"
	"github.com/imrx44/redbrut/internal/worker"
)

func main() {
	var (
		targetsFile = flag.String("t", "", "File with IP:PORT per line")
		host        = flag.String("H", "", "Single target (IP or IP:PORT)")
		usersFile   = flag.String("u", "", "File with usernames")
		passFile    = flag.String("p", "", "File with passwords")
		concurrency = flag.Int("c", 5000, "Goroutine pool size")
		ratePerIP   = flag.Float64("r", 5, "Max attempts/sec per IP")
		outFile     = flag.String("o", "goods.txt", "Output file for found credentials")
		timeout     = flag.Int("T", 5, "Per-attempt timeout in seconds")
		spray       = flag.Bool("spray", false, "Password spray mode")
		resume      = flag.Bool("resume", false, "Resume from previous session")
		jsonMode    = flag.Bool("json", false, "Output JSON lines")
		noRetry     = flag.Bool("no-retry", false, "Disable retry on network errors")
		maxRetries  = flag.Int("retries", 3, "Max retries per job")
	)
	flag.Parse()

	if (*targetsFile == "" && *host == "") || *usersFile == "" || *passFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: redbrut -t targets.txt -u users.txt -p passwords.txt [flags]\n")
		fmt.Fprintf(os.Stderr, "       redbrut -H 192.168.1.10 -u users.txt -p passwords.txt [flags]\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load inputs
	var targets []input.Target
	if *host != "" {
		t, err := input.ParseTarget(*host)
		dieOn(err, "parse host")
		targets = []input.Target{t}
	} else {
		var err error
		targets, err = input.LoadTargets(*targetsFile)
		dieOn(err, "load targets")
	}

	users, err := input.LoadLines(*usersFile)
	dieOn(err, "load users")

	passwords, err := input.LoadLines(*passFile)
	dieOn(err, "load passwords")

	if len(targets) == 0 || len(users) == 0 || len(passwords) == 0 {
		fmt.Fprintln(os.Stderr, "Error: targets, users, and passwords must all be non-empty")
		os.Exit(1)
	}

	resumePath := *outFile + ".resume"
	var doneJobs map[string]bool
	if *resume {
		doneJobs, err = output.LoadResumeState(resumePath)
		dieOn(err, "load resume state")
		fmt.Printf("[*] Resuming — skipping %d completed jobs\n", len(doneJobs))
	} else {
		doneJobs = make(map[string]bool)
	}

	reporter, err := output.NewReporter(*outFile, resumePath, *jsonMode)
	dieOn(err, "create reporter")
	defer reporter.Close()

	s := &stats.Stats{}
	totalJobs := int64(len(targets)) * int64(len(users)) * int64(len(passwords))
	s.Total = totalJobs

	fmt.Printf("[*] Targets: %d  Users: %d  Passwords: %d  Total: %d combos\n",
		len(targets), len(users), len(passwords), totalJobs)
	fmt.Printf("[*] Concurrency: %d  Rate/IP: %.0f/s  Timeout: %ds\n",
		*concurrency, *ratePerIP, *timeout)
	fmt.Printf("[*] Output: %s\n\n", *outFile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n[!] Interrupted — saving state for resume...")
		cancel()
	}()

	// Worker pool
	pool := worker.NewPool(worker.Config{
		Concurrency:  *concurrency,
		RatePerIP:    *ratePerIP,
		Timeout:      time.Duration(*timeout) * time.Second,
		LockoutPause: 30 * time.Minute,
		MaxRetries:   *maxRetries,
		NoRetry:      *noRetry,
	}, reporter, s, doneJobs)

	// Generate jobs
	mode := input.ModeCredential
	if *spray {
		mode = input.ModeSpray
	}
	jobs := input.GenerateCombos(targets, users, passwords, mode)

	// TUI
	tui := output.NewTUI(s, len(targets), *concurrency, *ratePerIP)

	// Run pool in background, TUI in foreground
	go func() {
		pool.Run(ctx, jobs)
		tui.SetDone()
	}()

	prog := tea.NewProgram(tui, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		// Fallback if TUI fails — plain output
		fmt.Fprintf(os.Stderr, "TUI error: %v — running without TUI\n", err)
		select {
		case <-ctx.Done():
		}
	}

	fmt.Printf("\n[+] Done. Found %d valid credentials. Saved to %s\n",
		s.Found.Load(), *outFile)
}

func dieOn(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %s: %v\n", msg, err)
		os.Exit(1)
	}
}
