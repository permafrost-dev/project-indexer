package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type GithubUser struct {
	Name string `json:"name"`
}

type GithubOrganization struct {
	Login                   string      `json:"login"`
	ID                      int         `json:"id"`
	NodeID                  string      `json:"node_id"`
	URL                     string      `json:"url"`
	ReposURL                string      `json:"repos_url"`
	EventsURL               string      `json:"events_url"`
	HooksURL                string      `json:"hooks_url"`
	IssuesURL               string      `json:"issues_url"`
	MembersURL              string      `json:"members_url"`
	PublicMembersURL        string      `json:"public_members_url"`
	AvatarURL               string      `json:"avatar_url"`
	Description             string      `json:"description"`
	Name                    string      `json:"name"`
	Company                 interface{} `json:"company"`
	Blog                    string      `json:"blog"`
	Location                string      `json:"location"`
	Email                   string      `json:"email"`
	TwitterUsername         string      `json:"twitter_username"`
	IsVerified              bool        `json:"is_verified"`
	HasOrganizationProjects bool        `json:"has_organization_projects"`
	HasRepositoryProjects   bool        `json:"has_repository_projects"`
	PublicRepos             int         `json:"public_repos"`
	PublicGists             int         `json:"public_gists"`
	Followers               int         `json:"followers"`
	Following               int         `json:"following"`
	HTMLURL                 string      `json:"html_url"`
	CreatedAt               time.Time   `json:"created_at"`
	UpdatedAt               time.Time   `json:"updated_at"`
	Type                    string      `json:"type"`
}

type GithubOrg struct {
	Login            string `json:"login"`
	ID               int    `json:"id"`
	NodeID           string `json:"node_id"`
	URL              string `json:"url"`
	ReposURL         string `json:"repos_url"`
	EventsURL        string `json:"events_url"`
	HooksURL         string `json:"hooks_url"`
	IssuesURL        string `json:"issues_url"`
	MembersURL       string `json:"members_url"`
	PublicMembersURL string `json:"public_members_url"`
	AvatarURL        string `json:"avatar_url"`
	Description      string `json:"description"`
}

func IsYes(input string) bool {
	lowerInput := strings.ToLower(input)
	return lowerInput == "yes" || lowerInput == "ye" || lowerInput == "y"
}

func callFuncWithStatus(message string, withCheckmark bool, handler func()) {
	fmt.Printf("%s...", message)

	handler()

	if withCheckmark {
		fmt.Println(" \u2713")
		return
	}

	fmt.Println(" done.")
}

func GetGithubOrganizationName(orgName string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/orgs/%s", orgName)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var org GithubOrganization
	err = json.Unmarshal(body, &org)
	if err != nil {
		return "", err
	}

	if org.Name == "" {
		return "", errors.New("no name found")
	}

	return org.Name, nil
}

func GetGithubUsernameFromGithubCli() (string, error) {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")

	var out strings.Builder
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

func getGithubUsernameFromGitRemote() (string, error) {
	out, err := gitCommand("config remote.origin.url")
	if err != nil {
		return "", err
	}

	remoteUrlParts := strings.Split(strings.Replace(strings.TrimSpace(out), ":", "/", -1), "/")
	return remoteUrlParts[1], nil
}

func getGitLogLines() ([]string, error) {
	//"--author='@users.noreply.github.com'",
	cmd := exec.Command("git", "log", "--pretty='%an:%ae'", "--reverse")

	var out strings.Builder
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	return lines, nil
}

func searchCommitsForGithubUsername() (string, error) {
	out, err := gitCommand(`config user.name`)

	if out == "" {
		return "", err
	}

	authorName := strings.ToLower(strings.TrimSpace(out))

	lines, _ := getGitLogLines()

	type committer struct {
		name, email string
		username    string
	}

	committers := make([]committer, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		username := strings.Split(parts[1], "@")[0]
		committers = append(committers, committer{parts[0], parts[1], username})
	}

	committerList := make([]committer, 0)

	for _, committer := range committers {
		if strings.Contains(committer.name, "[bot]") {
			continue
		}

		if strings.EqualFold(committer.name, authorName) {
			committerList = append(committerList, committer)
		}
	}

	if len(committerList) == 0 {
		return "", nil
	}

	return committerList[0].username, nil
}

func guessGithubUsername() (string, error) {
	result, err := searchCommitsForGithubUsername()

	if err != nil {
		return "", err
	}

	if result != "" {
		return result, nil
	}

	result, _ = GetGithubUsernameFromGithubCli()

	if result != "" {
		return result, nil
	}

	result, err = getGithubUsernameFromGitRemote()

	if err != nil {
		return "", err
	}

	return result, nil
}

func gitCommand(args string) (string, error) {
	cmd := exec.Command("git", strings.Split(args, " ")...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func GetGithubUserName(username string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s", username)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var user GithubUser
	err = json.Unmarshal(body, &user)
	if err != nil {
		return "", err
	}

	if user.Name == "" {
		return "", errors.New("no name found")
	}

	return user.Name, nil
}

func getGithubUserFirstOrg(username string) (GithubOrg, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/users/%s/orgs", username))

	if err != nil {
		return GithubOrg{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GithubOrg{}, err
	}

	var orgs []GithubOrg
	err = json.Unmarshal(body, &orgs)
	if err != nil {
		return GithubOrg{}, err
	}

	if len(orgs) > 0 {
		return orgs[0], nil
	}

	return GithubOrg{}, fmt.Errorf("no organizations found")
}

func GetGithubVendorUsername(username string) (string, error) {
	org, _ := getGithubUserFirstOrg(username)

	if org != (GithubOrg{}) {
		return org.Login, nil
	}

	output, err := exec.Command("git", "remote", "get-url", "origin").Output()

	if err != nil {
		return "", err
	}

	url := strings.Trim(string(output), " \t\r\n")

	re := regexp.MustCompile(`(?i)(?:github\.com[:/])([\w-]+/[\w-]+)`)

	matches := re.FindStringSubmatch(url)
	var result string

	if len(matches) > 1 {
		result = strings.Split(matches[1], "/")[0]
		orgName, err := GetGithubOrganizationName(result)

		if err == nil {
			result = orgName
		}
	} else {
		return "", errors.New("could not find github username")
	}

	return result, nil
}

func promptUserForInput(prompt string, defaultValue string) string {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if defaultValue != "" {
			fmt.Printf("%s (%s) ", prompt, defaultValue)
		} else {
			fmt.Printf("%s ", prompt)
		}

		scanner.Scan()

		input := strings.TrimSpace(scanner.Text())

		if input == "" && defaultValue != "" {
			return defaultValue
		}

		if input != "" {
			return input
		}
	}
}

func stringInArray(str string, arr []string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

func removeBetween(str, start, end string) string {
	for {
		s := strings.Index(str, start)
		e := strings.Index(str, end)
		// If start or end string is not found, return the original string
		if s == -1 || e == -1 {
			return str
		}
		// Remove text between start and end string
		str = str[:s] + str[e+len(end):]
	}
}

func processReadmeFile() {
	content, err := os.ReadFile("README.md")
	if err != nil {
		return
	}

	str := removeBetween(string(content), "<!-- ==START TEMPLATE README== -->", "<!-- ==END TEMPLATE README== -->")

	os.WriteFile("README.md", []byte(str), 0644)
}

func installGitHooks() {
	bytes, err := os.ReadFile(".git/config")
	if err != nil {
		return
	}

	content := string(bytes)

	if strings.Contains(string(content), "hooksPath") {
		return
	}

	content = strings.Replace(content, "[core]", "[core]\n\thooksPath = .custom-hooks", 1)

	os.WriteFile(".git/config", []byte(content), 0644)
}

func processDirectoryFiles(dir string, varMap map[string]string) {
	// get the files in the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
		return
	}

	ignoreFiles := []string{
		".git",
		".gitattributes",
		".gitignore",
		"configure-project.go",
		"build-all.go",
		"build-version.go",
		"go.sum",
	}

	// loop through the files
	for _, file := range files {
		if stringInArray(strings.ToLower(file.Name()), ignoreFiles) {
			continue
		}

		filePath := dir + "/" + file.Name()

		if file.IsDir() {
			processDirectoryFiles(filePath, varMap)
			continue
		}

		bytes, err := os.ReadFile(filePath)

		if err != nil {
			fmt.Println(err)
			continue
		}

		content := string(bytes)

		for key, value := range varMap {
			if file.Name() == "go.mod" {
				tempKey := strings.ReplaceAll(key, ".", "-")
				content = strings.ReplaceAll(content, "/"+tempKey, "/"+value)
				continue
			}

			key = "{{" + key + "}}"
			content = strings.ReplaceAll(content, key, value)
		}

		if string(bytes) != content {
			fmt.Printf("Updating file: %s\n", filePath)
			os.WriteFile(filePath, []byte(content), 0644)
		}
	}
}

func removeDirectoryRecursive(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, file := range files {
		filePath := dir + "/" + file.Name()

		if file.IsDir() {
			removeDirectoryRecursive(filePath)
			continue
		}

		os.Remove(filePath)
	}

	os.RemoveAll(dir)
}

func removeAssetsDir() {
	removeDirectoryRecursive("./assets")
}

func removeConfigureScript() {
	removeDirectoryRecursive("./tools/configure")
}

func setupCobra() {
	installCobraCLI := func() {
		_, err := exec.LookPath("cobra-cli")
		if err == nil {
			return
		}

		fmt.Printf("Installing Cobra CLI...\n")

		cmd := exec.Command("go", "install", "github.com/spf13/cobra-cli@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Error while installing cobra: %v\n", err)
		} else {
			fmt.Printf("Done.\n")
		}
	}

	initializeCobraPackage := func() {
		fmt.Printf("Initializing Cobra package...\n")

		cmd := exec.Command("cobra-cli", "init")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Error while initializing cobra: %v\n", err)
		} else {
			fmt.Printf("Done.\n")
		}
	}

	installCobraCLI()
	initializeCobraPackage()
}

func getCurrentGoVersion() string {
	defaultVersion := "1.20"

	out, err := exec.Command("go", "version").Output()

	if err != nil {
		return defaultVersion
	}

	re := regexp.MustCompile(`go(\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(out))

	if len(matches) > 1 {
		return matches[1]
	}

	return defaultVersion
}

func patternReplace(input string, pattern string, replacement string) string {
	return regexp.MustCompile(pattern).ReplaceAllString(input, replacement)
}

func updateGoModFile(projectDir string, varMap map[string]string) {
	data, err := os.ReadFile(projectDir + "/go.mod")
	if err != nil {
		fmt.Println(err)
		return
	}

	content := string(data)
	content = patternReplace(content, `module github.com/vendor-name/project-name`, "module github.com/"+varMap["project.vendor.github"]+"/"+varMap["project.name"])
	content = patternReplace(content, `go [0-9]+\.[0-9]+`, "go "+getCurrentGoVersion())

	os.WriteFile(projectDir+"/go.mod", []byte(content), 0644)
}

func getGitUsernameAndEmail() (string, string) {
	githubNameBytes, err := exec.Command("git", "config", "--global", "user.name").Output()
	if err != nil {
		githubNameBytes = []byte("")
	}

	githubEmailBytes, err := exec.Command("git", "config", "--global", "user.email").Output()
	if err != nil {
		githubEmailBytes = []byte("")
	}

	githubName := strings.Trim(string(githubNameBytes), " \r\n\t")
	githubEmail := strings.Trim(string(githubEmailBytes), " \r\n\t")

	return githubName, githubEmail
}

func generateAppVersionFile(projectDir string) {
	appDir := projectDir + "/app"
	os.MkdirAll(appDir, 0755)
	os.WriteFile(appDir+"/version.go", []byte(`package main\n\nvar Version = "0.0.0"\n`), 0644)
}

func main() {
	// get the current directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	projectDir, err := filepath.Abs(cwd)

	if err != nil {
		fmt.Println(err)
		return
	}

	varMap := make(map[string]string)

	githubName, githubEmail := getGitUsernameAndEmail()
	githubUser, _ := guessGithubUsername()

	varMap["date.year"] = fmt.Sprintf("%d", time.Now().Local().Year())
	varMap["project.name.full"] = promptUserForInput("Project name: ", path.Base(projectDir))
	varMap["project.name"] = strings.ReplaceAll(varMap["project.name.full"], " ", "-")
	varMap["project.description"] = promptUserForInput("Project description: ", "")
	varMap["project.author.name"] = promptUserForInput("Your full name: ", githubName)
	varMap["project.author.email"] = promptUserForInput("Your email address: ", githubEmail)
	varMap["project.author.github"] = promptUserForInput("Your github username: ", githubUser)

	vendorUsername, _ := GetGithubVendorUsername(varMap["project.author.github"])
	varMap["project.vendor.github"] = promptUserForInput("User/org vendor github name: ", vendorUsername)

	vendorName, _ := GetGithubUserName(varMap["project.vendor.github"])
	varMap["project.vendor.name"] = promptUserForInput("User/org vendor name: ", vendorName)

	varMap["packages.cobra"] = promptUserForInput("Use spf13/cobra? (Y/n): ", "y")

	updateGoModFile(projectDir, varMap) // must be called before processing files
	processDirectoryFiles(projectDir, varMap)
	processReadmeFile()
	generateAppVersionFile(projectDir)

	callFuncWithStatus("Installing git hooks", true, installGitHooks)
	callFuncWithStatus("Removing assets directory", true, removeAssetsDir)
	callFuncWithStatus("Removing configure script", true, removeConfigureScript)

	if IsYes(varMap["packages.cobra"]) {
		setupCobra()
	}

	fmt.Println("Done!")
}
