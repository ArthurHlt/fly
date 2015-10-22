package commands

import (
	"fmt"
	"log"
	"sort"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/fatih/color"
)

type ContainersCommand struct{}

var containersCommand ContainersCommand

func init() {
	containers, err := Parser.AddCommand(
		"containers",
		"Print the running containers",
		"",
		&containersCommand,
	)
	if err != nil {
		panic(err)
	}

	containers.Aliases = []string{"cs"}
}

func (command *ContainersCommand) Execute([]string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln(err)
	}
	handler := atcclient.NewAtcHandler(client)

	containers, err := handler.ListContainers()
	if err != nil {
		log.Fatalln(err)
	}

	headers := TableRow{
		{Contents: "handle", Color: color.New(color.Bold)},
		{Contents: "name", Color: color.New(color.Bold)},
		{Contents: "pipeline", Color: color.New(color.Bold)},
		{Contents: "type", Color: color.New(color.Bold)},
		{Contents: "build id", Color: color.New(color.Bold)},
		{Contents: "worker", Color: color.New(color.Bold)},
	}

	table := Table{Data: []TableRow{headers}}

	sort.Sort(byHandle(containers))

	for _, c := range containers {
		row := TableRow{
			{Contents: c.ID},
			{Contents: c.Name},
			{Contents: c.PipelineName},
			{Contents: c.Type},
			buildIDOrNone(c.BuildID),
			{Contents: c.WorkerName},
		}

		table.Data = append(table.Data, row)
	}

	fmt.Print(table.Render())

	return nil
}

type byHandle []atc.Container

func (cs byHandle) Len() int               { return len(cs) }
func (cs byHandle) Swap(i int, j int)      { cs[i], cs[j] = cs[j], cs[i] }
func (cs byHandle) Less(i int, j int) bool { return cs[i].ID < cs[j].ID }

func buildIDOrNone(id int) TableCell {
	var column TableCell

	if id == 0 {
		column.Contents = "none"
		column.Color = color.New(color.Faint)
	} else {
		column.Contents = strconv.Itoa(id)
	}

	return column
}
