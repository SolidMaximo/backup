package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const JSON_FILENAME = "backup_tool.json"

type RobotNamelist []string

func (r *RobotNamelist) String() string {
	return fmt.Sprint(*r)
}

func (r *RobotNamelist) Set(value string) error {
	if len(*r) > 0 {
		return errors.New("robot namelist flag already been set")
	}

	for _, n := range strings.Split(value, ",") {
		*r = append(*r, n)
	}

	return nil
}

type Project struct {
	Destination string
	Version     string
	Robots      []Robot
}

func (p *Project) fromJSON() error {
	data, err := ioutil.ReadFile(JSON_FILENAME)
	if err != nil {
		return err
	}

	json.Unmarshal([]byte(data), p)

	return nil
}

func (p *Project) fromWizard() error {
	r := bufio.NewReader(os.Stdin)

questions:
	fmt.Println("Where should backups be stored?")
	dest, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	dest = strings.TrimSpace(dest)

confirm:
	fmt.Printf("Destination: %s\n", dest)
	fmt.Println("Is this correct? (Y/N)")

	answer, err := r.ReadString('\n')
	if err != nil {
		return err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	switch answer {
	case "y":
		// noop
	case "n":
		goto questions
	default:
		goto confirm
	}

	p.Destination = dest
	p.Version = VERSION

	return p.Save()
}

func NewProject() (*Project, error) {
	p := &Project{}

	err := p.fromJSON()
	if os.IsNotExist(err) {
		err = p.fromWizard()
	}
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Project) Save() error {
	p.Version = VERSION

	b, err := json.Marshal(p)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(JSON_FILENAME, b, 0644)
	if err != nil {
		return err
	}

	fmt.Println("Project saved.")

	return nil
}

func (p *Project) AddRobot() error {
	r, err := NewRobot()
	if err != nil {
		return err
	}

	p.Robots = append(p.Robots, *r)
	return p.Save()
}

func (p *Project) RemoveRobot() error {
	if len(p.Robots) < 1 {
		fmt.Println("Your project does not have any robots. Please run `BackupTool add` to add one.")
		return nil
	}

list:
	for id, robot := range p.Robots {
		fmt.Printf("%d. %s %s\n", id+1, robot.Name, robot.Host)
	}

	fmt.Println("\nWhich robot do you want to remove?")

	var id int
	_, err := fmt.Scanf("%d", &id)
	if err != nil {
		fmt.Println("Invalid id. Try again.")
		goto list
	}

	id = id - 1
	if id < 0 || id > len(p.Robots)-1 {
		fmt.Println("Id out of range")
		goto list
	}

	fmt.Printf("Removing robot #%d\n", id+1)

	p.Robots = append(p.Robots[:id], p.Robots[id+1:]...)

	return p.Save()
}

func (p *Project) filteredRobots(namelist RobotNamelist) []Robot {
	if len(namelist) == 0 {
		return p.Robots
	}

	var l []Robot
	for _, n := range namelist {
		for _, r := range p.Robots {
			if r.Name == n {
				l = append(l, r)
			}
		}
	}

	return l
}
