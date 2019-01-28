package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"
)

func convertFromKebabCase(kebab string) string {
	var sb strings.Builder
	up := false
	for _, r := range kebab {
		switch up {
		case true:
			fmt.Fprint(&sb, strings.ToUpper(string(r)))
			up = false
		case false:
			if r == '-' {
				up = true
			} else {
				fmt.Fprint(&sb, string(r))
			}
		}
	}

	return sb.String()
}

type prop struct {
	Method   string
	Property string
}

type event struct {
	Channel string
	Event   string
}

type command struct {
	Method  string
	Command string
}

func main() {
	var props []prop
	{
		f, err := os.Open("gen/properties")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())

			if line == "" {
				continue
			}

			if line[0] == '#' {
				continue
			}

			props = append(props, prop{
				Method:   strings.Title(convertFromKebabCase(line)),
				Property: line,
			})
		}
	}

	var events []event
	{
		f, err := os.Open("gen/events")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())

			if line == "" {
				continue
			}

			if line[0] == '#' {
				continue
			}

			events = append(events, event{
				Channel: strings.Title(convertFromKebabCase(line)),
				Event:   line,
			})
		}
	}

	var commands []command
	{
		f, err := os.Open("gen/commands")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())

			if line == "" {
				continue
			}

			if line[0] == '#' {
				continue
			}

			commands = append(commands, command{
				Method:  strings.Title(convertFromKebabCase(line)),
				Command: line,
			})
		}
	}

	{
		f, err := os.Create("properties.go")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		fmt.Println("[gen] properties.go")
		propertiesTmpl.Execute(f, struct {
			Timestamp  time.Time
			Properties []prop
		}{
			Timestamp:  time.Now(),
			Properties: props,
		})
	}

	{
		f, err := os.Create("events.go")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		fmt.Println("[gen] events.go")
		eventsTmpl.Execute(f, struct {
			Timestamp time.Time
			Events    []event
		}{
			Timestamp: time.Now(),
			Events:    events,
		})
	}

	{
		f, err := os.Create("commands.go")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		fmt.Println("[gen] commands.go")
		commandsTmpl.Execute(f, struct {
			Timestamp time.Time
			Commands  []command
		}{
			Timestamp: time.Now(),
			Commands:  commands,
		})
	}
}

var propertiesTmpl = template.Must(template.New("").Parse(`// Code generated by go generate; DO NOT EDIT.
// {{ .Timestamp }}
package mpv

import (
	"fmt"
)

// property getters
func (c *Client) GetProperty(name string) (prop interface{}, err error) {
	ch, err := c.Execute("get_property", name)
	reply, err := ReplyOrTimeout(ch, WaitForReply)
	if err != nil {
		err = fmt.Errorf("mpv.GetProperty: '%s': %s", name, err)
		return
	}
	if reply.Error != "success" {
		err = fmt.Errorf("mpv.GetProperty: '%s': %s", name, reply.Error)
		return
	}
	prop = reply.Data

	return
}
{{ range .Properties }}
// {{.Method}} gets '{{.Property}}' property
func (c *Client) {{.Method}}() (prop interface{}, err error) {
	return c.GetProperty("{{.Property}}")
}
{{ end }}
// property setters
func (c *Client) SetProperty(name string, value interface{}) (err error) {
	ch, err := c.Execute("set_property", name, value)
	reply, err := ReplyOrTimeout(ch, WaitForReply)
	if err != nil {
		err = fmt.Errorf("mpv.SetProperty: '%s': %s", name, err)
		return
	}
	if reply.Error != "success" {
		err = fmt.Errorf("mpv.SetProperty: '%s': %s", name, reply.Error)
		return
	}

	return
}
{{ range .Properties }}
// Set{{.Method}} sets '{{.Property}}' property
func (c *Client) Set{{.Method}}(value interface{}) error {
	return c.SetProperty("{{.Property}}", value)
}
{{ end }}
`))

var eventsTmpl = template.Must(template.New("").Parse(`// Code generated by go generate; DO NOT EDIT.
// {{ .Timestamp }}
package mpv

import ()

func handleEvent(ev string) {
	go func() {
		switch ev { {{ range .Events }}
		case "{{.Event}}":
			{{.Channel}} <- struct{}{}{{ end }}
		default:
			Log("mpv.handleEvent: unknown event name: %s", ev)
		}
	}()
}
{{ range .Events }}
// {{.Channel}} is the channel for the '{{.Event}}' event
var {{.Channel}} = make(chan struct{}, 1)
{{ end }}
`))

var commandsTmpl = template.Must(template.New("").Parse(`// Code generated by go generate; DO NOT EDIT.
// {{ .Timestamp }}
package mpv

import (
	"fmt"
)

// SendCommand using Execute then wait for a reply
func (c *Client) SendCommand(name string, args ...interface{}) error {
	ch, err := c.Execute(name, args...)
	reply, err := ReplyOrTimeout(ch, WaitForReply)
	if err != nil {
		return fmt.Errorf("mpv.SendCommand: '%s': %s", name, err)
	}
	if reply.Error != "success" {
		return fmt.Errorf("mpv.SendCommand: '%s': %s", name, reply.Error)
	}

	return nil
}

{{ range .Commands }}
// {{.Method}} runs '{{.Command}}' command
func (c *Client) {{.Method}}(params ...interface{}) error {
	return c.SendCommand("{{.Command}}", params...)
}
{{ end }}
`))
