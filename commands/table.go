package commands

import (
	"strings"

	"github.com/fatih/color"
)

type Table struct {
	Headers TableRow
	Data    []TableRow
}

type TableRow []TableCell

type TableCell struct {
	Contents string
	Color    *color.Color
}

func (table Table) Render() string {
	columnWidths := map[int]int{}

	for _, row := range table.Data {
		for i, column := range row {
			columnWidth := len(column.Contents)

			if columnWidth > columnWidths[i] {
				columnWidths[i] = columnWidth
			}
		}
	}

	for i, column := range table.Headers {
		columnWidth := len(column.Contents)

		if columnWidth > columnWidths[i] {
			columnWidths[i] = columnWidth
		}
	}

	output := ""

	for i, column := range table.Headers {
		contents := column.Contents
		if column.Color != nil {
			contents = column.Color.SprintFunc()(contents)
		}

		output += contents

		paddingSize := columnWidths[i] - len(column.Contents)
		output += strings.Repeat(" ", paddingSize)

		if i+1 < len(columnWidths) {
			output += "  "
		}
	}

	output += "\n"

	for _, row := range table.Data {
		for i, column := range row {
			contents := column.Contents
			if column.Color != nil {
				contents = column.Color.SprintFunc()(contents)
			}

			output += contents

			paddingSize := columnWidths[i] - len(column.Contents)
			output += strings.Repeat(" ", paddingSize)

			if i+1 < len(columnWidths) {
				output += "  "
			}
		}

		output += "\n"
	}

	return output
}
