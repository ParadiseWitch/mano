package store

import (
	"bytes"
	"fmt"
	"strings"
)

// Serialize renders the journal back to org-mode. Days are written in the order
// they are held, which Parse and Save both keep ascending.
func (j Journal) Serialize() []byte {
	var b bytes.Buffer

	for i, day := range j.Days {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "* %s\n", day.Date)

		for _, item := range day.Items {
			b.WriteString("** ")
			if item.Todo != "" {
				b.WriteString(item.Todo)
				b.WriteByte(' ')
			}
			b.WriteString(item.Content)
			if len(item.Tags) > 0 {
				b.WriteString("  :")
				b.WriteString(strings.Join(item.Tags, ":"))
				b.WriteString(":")
			}
			b.WriteByte('\n')
			if item.Start != nil || item.End != nil {
				if item.Start != nil {
					fmt.Fprintf(&b, "   - START: %s\n", item.Start)
				}
				if item.End != nil {
					fmt.Fprintf(&b, "   - END: %s\n", item.End)
				}
			}
		}
	}

	return b.Bytes()
}
