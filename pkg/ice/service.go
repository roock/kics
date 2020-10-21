package ice

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/checkmarxDev/ice/pkg/engine"
	"github.com/checkmarxDev/ice/pkg/model"
	"github.com/checkmarxDev/ice/pkg/parser"
	"github.com/checkmarxDev/ice/pkg/source"
	"github.com/pkg/errors"
)

type SourceProvider interface {
	GetSources(ctx context.Context, scanID string, extensions model.Extensions, sink source.Sink) error
}

type Storage interface {
	SaveFile(ctx context.Context, metadata *model.FileMetadata) error
	SaveVulnerabilities(ctx context.Context, vulnerabilities []model.Vulnerability) error
	GetVulnerabilities(ctx context.Context, scanID string) ([]model.Vulnerability, error)
	GetScanSummary(ctx context.Context, scanIDs []string) ([]model.SeveritySummary, error)
}

type Tracker interface {
	TrackFileFound()
	TrackFileParse()
}

type Service struct {
	SourceProvider SourceProvider
	Storage        Storage
	Parser         *parser.Parser
	Inspector      *engine.Inspector
	Tracker        Tracker
}

func (s *Service) StartScan(ctx context.Context, scanID string) error {
	var files model.FileMetadatas
	if err := s.SourceProvider.GetSources(
		ctx,
		scanID,
		s.Parser.SupportedExtensions(),
		func(ctx context.Context, filename string, rc io.ReadCloser) error {
			s.Tracker.TrackFileFound()

			content, err := ioutil.ReadAll(rc)
			if err != nil {
				return errors.Wrap(err, "failed to read file content")
			}

			documents, kind, err := s.Parser.Parse(filename, content)
			if err != nil {
				return errors.Wrap(err, "failed to parse file content")
			}

			for _, document := range documents {
				file := model.FileMetadata{
					ScanID:       scanID,
					Document:     document,
					OriginalData: string(content),
					Kind:         kind,
					FileName:     filename,
				}

				err = s.Storage.SaveFile(ctx, &file)
				if err == nil {
					files = append(files, file)
					s.Tracker.TrackFileParse()
				}
			}

			return errors.Wrap(err, "failed to save file content")
		},
	); err != nil {
		return errors.Wrap(err, "failed to read sources")
	}

	vulnerabilities, err := s.Inspector.Inspect(ctx, scanID, files)
	if err != nil {
		return errors.Wrap(err, "failed to inspect files")
	}

	err = s.Storage.SaveVulnerabilities(ctx, vulnerabilities)

	return errors.Wrap(err, "failed to save vulnerabilities")
}

func (s *Service) GetVulnerabilities(ctx context.Context, scanID string) ([]model.Vulnerability, error) {
	return s.Storage.GetVulnerabilities(ctx, scanID)
}

func (s *Service) GetScanSummary(ctx context.Context, scanIDs []string) ([]model.SeveritySummary, error) {
	return s.Storage.GetScanSummary(ctx, scanIDs)
}
