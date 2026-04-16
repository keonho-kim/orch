package session

import (
	"context"

	"github.com/keonho-kim/orch/domain"
)

func (s *Service) SyncMirror() error {
	if s.mirror == nil {
		return nil
	}

	metadata, err := s.manager.ListMetadata(0)
	if err != nil {
		return err
	}
	for _, meta := range metadata {
		if err := s.syncMetadata(meta); err != nil {
			return err
		}
		records, err := s.manager.LoadRecords(meta.SessionID)
		if err != nil {
			return err
		}
		for _, record := range records {
			if err := s.syncRecord(record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) syncMetadata(meta domain.SessionMetadata) error {
	if s.mirror == nil {
		return nil
	}
	return s.mirror.UpsertSession(context.Background(), meta)
}

func (s *Service) syncRecord(record domain.SessionRecord) error {
	if s.mirror == nil {
		return nil
	}
	return s.mirror.AppendSessionMessage(context.Background(), record)
}
