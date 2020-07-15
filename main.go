package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"log"
	"path/filepath"
	"strings"
)

const ShellToUse = "bash"

type pattern struct {
	Flags    string   `json:"flags,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
	Patterns []string `json:"patterns,omitempty"`
	Engine   string   `json:"engine,omitempty"`
}

func main() {
	var saveMode bool
	flag.BoolVar(&saveMode, "save", false, "save a pattern (e.g: gf -save pat-name -Hnri 'search-pattern')")

	var listMode bool
	flag.BoolVar(&listMode, "list", false, "list available patterns")

	var dumpMode bool
	flag.BoolVar(&dumpMode, "dump", false, "prints the grep command rather than executing it")

	flag.Parse()

	if listMode {
		pats, err := getPatterns()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}

		fmt.Println(strings.Join(pats, "\n"))
		return
	}

	if saveMode {
		name := flag.Arg(0)
		flags := flag.Arg(1)
		pattern := flag.Arg(2)

		err := savePattern(name, flags, pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		return
	}

	patName := flag.Arg(0)
	files := flag.Arg(1)
	if files == "" {
		pwd, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
		}
		files = pwd + "/*"
	}

	patDir, err := getPatternDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to open user's pattern directory")
		return
	}

	filename := filepath.Join(patDir, patName+".json")
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no such pattern")
		return
	}
	defer f.Close()

	pat := pattern{}
	dec := json.NewDecoder(f)
	err = dec.Decode(&pat)

	if err != nil {
		fmt.Fprintf(os.Stderr, "pattern file '%s' is malformed: %s\n", filename, err)
		return
	}

	if pat.Pattern == "" {
		// check for multiple patterns
		if len(pat.Patterns) == 0 {
			fmt.Fprintf(os.Stderr, "pattern file '%s' contains no pattern(s)\n", filename)
			return
		}

		pat.Pattern = "(" + strings.Join(pat.Patterns, "|") + ")"
	}

		var cmd *exec.Cmd
		operator := "grep"
		if pat.Engine != "" {
			operator = pat.Engine
		}

		var cmd_string =  fmt.Sprintf(" %v %v %q %v ",operator, pat.Flags, pat.Pattern, files )
		var dump_string =  fmt.Sprintf("bash -c  %v %v %q %v ",operator, pat.Flags, pat.Pattern, files )
		
		if dumpMode {
				fmt.Printf(dump_string)
				fmt.Printf("\n")
				os.Exit(0)
		} 

		if stdinIsPipe() {
			cmd = exec.Command(operator, pat.Flags, pat.Pattern)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		
		} else {
			err, out, errout := Shellout(cmd_string)
			if err != nil {
				log.Printf("error: %v\n", err)
			}
			fmt.Println("--- stdout ---")
			fmt.Println(out)
			fmt.Println("--- stderr ---")
			fmt.Println(errout)
		}


}

func Shellout(command string) (error, string, string) {
    var stdout bytes.Buffer
    var stderr bytes.Buffer
    cmd := exec.Command(ShellToUse, "-c", command)
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    err := cmd.Run()
    return err, stdout.String(), stderr.String()
}


func getPatternDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	path := filepath.Join(usr.HomeDir, ".config/gf")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		// .config/gf exists
		return path, nil
	}
	return filepath.Join(usr.HomeDir, ".gf"), nil
}

func savePattern(name, flags, pat string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}

	if pat == "" {
		return errors.New("pattern cannot be empty")
	}

	p := &pattern{
		Flags:   flags,
		Pattern: pat,
	}

	patDir, err := getPatternDir()
	if err != nil {
		return fmt.Errorf("failed to determine pattern directory: %s", err)
	}

	path := filepath.Join(patDir, name+".json")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return fmt.Errorf("failed to create pattern file: %s", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")

	err = enc.Encode(p)
	if err != nil {
		return fmt.Errorf("failed to write pattern file: %s", err)
	}

	return nil
}

func getPatterns() ([]string, error) {
	out := []string{}

	patDir, err := getPatternDir()
	if err != nil {
		return out, fmt.Errorf("failed to determine pattern directory: %s", err)
	}
	_ = patDir

	files, err := filepath.Glob(patDir + "/*.json")
	if err != nil {
		return out, err
	}

	for _, f := range files {
		f = f[len(patDir)+1 : len(f)-5]
		out = append(out, f)
	}

	return out, nil
}

func stdinIsPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
