package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// PDFGeneratorService handles PDF generation from images
type PDFGeneratorService struct {
	executor *utils.PythonExecutor
	logger   *utils.Logger
}

// NewPDFGeneratorService creates a new PDF generator service
func NewPDFGeneratorService(executor *utils.PythonExecutor, logger *utils.Logger) *PDFGeneratorService {
	return &PDFGeneratorService{
		executor: executor,
		logger:   logger,
	}
}

// GeneratePDF generates a PDF from multiple images
func (s *PDFGeneratorService) GeneratePDF(req models.PDFRequest) (models.PDFResult, error) {
	s.logger.Info("Generating PDF from %d images -> %s", len(req.ImagePaths), req.OutputPath)

	var result models.PDFResult
	err := s.executor.ExecuteAndParse("pdf_generator.py", req, &result)
	if err != nil {
		s.logger.Error("PDF generation failed: %v", err)
		return models.PDFResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("PDF generation failed: %s", result.Error)
		return result, fmt.Errorf("PDF generation failed: %s", result.Error)
	}

	s.logger.Info("PDF generated successfully: %d pages", result.PageCount)
	return result, nil
}
