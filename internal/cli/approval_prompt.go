package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func promptApproval(writer io.Writer, request *domain.ApprovalRequest) error {
	if _, err := fmt.Fprintf(writer, "\n[approval] tool=%s reason=%s\n%s\nApprove? [y/N]: ", request.Call.Name, request.Reason, request.Call.Arguments); err != nil {
		return fmt.Errorf("write approval prompt: %w", err)
	}
	return nil
}

func readApprovalDecision(reader *bufio.Reader, writer io.Writer) (bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read approval response: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		if _, err := io.WriteString(writer, "Denied.\n"); err != nil {
			return false, fmt.Errorf("write approval denial: %w", err)
		}
		return false, nil
	}
}
