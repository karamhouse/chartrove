package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/fastenhealth/fasten-sources/clients/factory"
	sourceModels "github.com/fastenhealth/fasten-sources/clients/models"
	sourceDefinitions "github.com/fastenhealth/fasten-sources/definitions"
	sourcePkg "github.com/fastenhealth/fasten-sources/pkg"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func CreateReconnectSource(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)

	sourceCred := models.SourceCredential{}
	if err := c.ShouldBindJSON(&sourceCred); err != nil {
		err = fmt.Errorf("an error occurred while parsing posted source credential: %s", err)
		logger.Errorln(err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	logger.Infof("Parsed Create SourceCredential Credentials Payload: %v", sourceCred)

	//get the endpoint definition
	endpointDefinition, err := sourceDefinitions.GetSourceDefinition(sourceDefinitions.GetSourceConfigOptions{
		EndpointId: sourceCred.EndpointID.String(),
	})

	if err != nil {
		err = fmt.Errorf("an error occurred while retrieving source definition: %w", err)
		logger.Errorln(err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if endpointDefinition.DynamicClientRegistrationMode == "user-authenticated" {
		logger.Warnf("This client requires a dynamic client registration, starting registration process")

		if len(endpointDefinition.RegistrationEndpoint) == 0 {
			err := fmt.Errorf("this client requires dynamic registration, but does not provide a registration endpoint: %s", endpointDefinition.DynamicClientRegistrationMode)
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		err := sourceCred.RegisterDynamicClient()
		if err != nil {
			err = fmt.Errorf("an error occurred while registering dynamic client: %w", err)
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		//generate a JWT token and then use it to get an access token for the dynamic client
		err = sourceCred.RefreshDynamicClientAccessToken()
		if err != nil {
			err = fmt.Errorf("an error occurred while retrieving access token for dynamic client: %w", err)
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	}

	if sourceCred.ID != uuid.Nil {
		//reconnect
		err := databaseRepo.UpdateSource(c, &sourceCred)
		if err != nil {
			err = fmt.Errorf("an error occurred while reconnecting source credential: %w", err)
			logger.Errorln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
	} else {
		//create source for the first time
		err := databaseRepo.CreateSource(c, &sourceCred)
		if err != nil {
			err = fmt.Errorf("an error occurred while storing source credential: %w", err)
			logger.Errorln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
	}

	// after creating the source, we should do a bulk import (in the background)

	summary, err := BackgroundJobSyncResources(GetBackgroundContext(c), logger, databaseRepo, &sourceCred)
	if err != nil {
		logger.Errorln(err)
		//errors from the background job will be wrapped and stored in the database, lets just return a generic error
		// this is also important because these errors:
		// 1. are not user facing - longer/scarier for users, and may show information that they are not equipped to resolve themselves.
		// 2. lots of duplicate text ("an error occurred while...") due to wrapping as the error bubbles up the codebase.
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "initial record sync failed. See background jobs page for more details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "source": sourceCred, "data": summary})
}

func SourceSync(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)
	eventBus := c.MustGet(pkg.ContextKeyTypeEventBusServer).(event_bus.Interface)

	logger.Infof("Get SourceCredential Credentials: %v", c.Param("sourceId"))

	sourceCred, err := databaseRepo.GetSource(c, c.Param("sourceId"))
	if err != nil {
		err = fmt.Errorf("an error occurred while retrieving source credential: %w", err)
		logger.Errorln(err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// after creating the source, we should do a bulk import (in the background)
	summary, err := BackgroundJobSyncResources(GetBackgroundContext(c), logger, databaseRepo, sourceCred)
	if err != nil {
		err := fmt.Errorf("an error occurred while syncing resources: %w", err)
		logger.Errorln(err)
		//errors from the background job will be wrapped and stored in the database, lets just return a generic error
		// this is also important because these errors:
		// 1. are not user facing - longer/scarier for users, and may show information that they are not equipped to resolve themselves.
		// 2. lots of duplicate text ("an error occurred while...") due to wrapping as the error bubbles up the codebase.
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "record sync failed. See background jobs page for more details"})
		return
	}

	//publish event
	currentUser, _ := databaseRepo.GetCurrentUser(c)
	err = eventBus.PublishMessage(
		models.NewEventSourceComplete(
			currentUser.ID.String(),
			sourceCred.ID.String(),
		),
	)
	if err != nil {
		logger.Warnf("ignoring: an error occurred while publishing sync complete event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "source": sourceCred, "data": summary})
}

// mimics functionality in CreateRelatedResources
// mimics functionality in SourceSync
func CreateManualSource(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)
	eventBus := c.MustGet(pkg.ContextKeyTypeEventBusServer).(event_bus.Interface)

	// store the bundle file locally
	bundleFile, err := storeFileLocally(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer func() {
		bundleFile.Close()
		os.Remove(bundleFile.Name())
	}()
	origin, err := directSMARTOriginFromForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	var directPatientID string
	if origin != nil {
		directPatientID, err = directSMARTPatientID(bundleFile)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		// Keep the completion check and write together for repeated confirmations.
		// The desktop already excludes concurrent processes for its data directory.
		directSMARTImportMu.Lock()
		defer directSMARTImportMu.Unlock()
	}

	// We cannot save the "SourceCredential" object yet, as we do not know the patientID

	// create a "manual" client, which we can use to parse the
	manualSourceCredential := models.SourceCredential{
		PlatformType: sourcePkg.PlatformTypeManual,
	}
	if origin != nil {
		manualSourceCredential.EndpointID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("chartrove-direct-smart:"+origin.FHIRBaseURL))
		manualSourceCredential.Display = fmt.Sprintf("Direct SMART snapshot: %s (%s)", origin.Display, origin.FHIRBaseURL)
	}
	tempSourceClient, err := factory.GetSourceClient("", c, logger, &manualSourceCredential)
	if err != nil {
		err = fmt.Errorf("an error occurred while initializing hub client using manual source without credentials: %w", err)
		logger.Errorln(err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	patientId, bundleType, err := tempSourceClient.ExtractPatientId(bundleFile)
	if err != nil {
		err = fmt.Errorf("an error occurred while extracting patient id: %w", err)
		logger.Errorln(err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	manualSourceCredential.Patient = patientId
	if origin != nil {
		// The legacy manual importer trims a character cutset from Patient IDs.
		// Direct snapshots keep the verified FHIR ID so distinct IDs cannot merge.
		manualSourceCredential.Patient = directPatientID
		sources, err := databaseRepo.GetSources(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "unable to check previous direct SMART imports"})
			return
		}
		for _, source := range sources {
			if source.PlatformType != sourcePkg.PlatformTypeManual || source.EndpointID != manualSourceCredential.EndpointID || source.Patient != directPatientID {
				continue
			}
			completed, err := directSMARTPatientExists(c, databaseRepo, source.ID.String(), directPatientID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "unable to check previous direct SMART snapshot"})
				return
			}
			if completed {
				c.JSON(http.StatusOK, gin.H{"success": true, "source": source, "already_imported": true})
				return
			}
			// A source alone is not proof of an imported Patient. Reuse its identity
			// after failed or interrupted persistence instead of reporting success.
			manualSourceCredential = source
			break
		}
	}

	//store the manualSourceCredential
	if manualSourceCredential.ID == uuid.Nil {
		err = databaseRepo.CreateSource(c, &manualSourceCredential)
	}
	if err != nil {
		err = fmt.Errorf("an error occurred while creating manual source: %w", err)
		logger.Errorln(err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	summary, err := BackgroundJobSyncResourcesWrapper(
		c,
		logger,
		databaseRepo,
		&manualSourceCredential,
		func(
			_backgroundJobContext context.Context,
			_logger *logrus.Entry,
			_databaseRepo database.DatabaseRepository,
			_sourceCred *models.SourceCredential,
		) (sourceModels.SourceClient, sourceModels.UpsertSummary, error) {
			manualSourceClient, err := factory.GetSourceClient("", _backgroundJobContext, _logger, _sourceCred)
			if err != nil {
				resultErr := fmt.Errorf("an error occurred while initializing hub client using manual source with credential: %w", err)
				logger.Errorln(resultErr)
				return manualSourceClient, sourceModels.UpsertSummary{}, resultErr
			}

			summary, err := manualSourceClient.SyncAllBundle(_databaseRepo, bundleFile, bundleType)
			if err != nil {
				resultErr := fmt.Errorf("an error occurred while processing bundle: %w", err)
				logger.Errorln(resultErr)
				return manualSourceClient, sourceModels.UpsertSummary{}, resultErr
			}
			if origin != nil {
				// SyncAllBundle may collect resource-write failures without returning
				// an error. A direct Patient-only snapshot must actually be present.
				persisted, err := directSMARTPatientExists(_backgroundJobContext, _databaseRepo, _sourceCred.ID.String(), directPatientID)
				if err != nil {
					return manualSourceClient, sourceModels.UpsertSummary{}, fmt.Errorf("unable to verify direct SMART snapshot: %w", err)
				}
				if !persisted {
					return manualSourceClient, sourceModels.UpsertSummary{}, fmt.Errorf("direct SMART Patient snapshot was not persisted; retry the import")
				}
			}
			return manualSourceClient, summary, nil
		})

	if err != nil {
		err = fmt.Errorf("an error occurred while storing manual source resources: %w", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	//publish event
	currentUser, _ := databaseRepo.GetCurrentUser(c)
	err = eventBus.PublishMessage(
		models.NewEventSourceComplete(
			currentUser.ID.String(),
			manualSourceCredential.ID.String(),
		),
	)
	if err != nil {
		logger.Warnf("ignoring: an error occurred while publishing sync complete event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary, "source": manualSourceCredential})

}

func GetSource(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)

	sourceCred, err := databaseRepo.GetSource(c, c.Param("sourceId"))
	if err != nil {
		logger.Errorln("An error occurred while retrieving source credential", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sourceCred})
}

func GetSourceSummary(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)

	sourceSummary, err := databaseRepo.GetSourceSummary(c, c.Param("sourceId"))
	if err != nil {
		logger.Errorln("An error occurred while retrieving source summary", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sourceSummary})
}

func ListSource(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)

	sourceCreds, err := databaseRepo.GetSources(c)
	if err != nil {
		logger.Errorln("An error occurred while listing source credentials", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sourceCreds})
}

func DeleteSource(c *gin.Context) {
	logger := c.MustGet(pkg.ContextKeyTypeLogger).(*logrus.Entry)
	databaseRepo := c.MustGet(pkg.ContextKeyTypeDatabase).(database.DatabaseRepository)

	rowsEffected, err := databaseRepo.DeleteSource(c, c.Param("sourceId"))
	if err != nil {
		logger.Errorln("An error occurred while deleting source credential", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rowsEffected})
}

// Helpers

type directSMARTOrigin struct {
	FHIRBaseURL string `json:"fhir_base_url"`
	Display     string `json:"display"`
}

var directSMARTImportMu sync.Mutex
var directSMARTPatientIDPattern = regexp.MustCompile(`^[A-Za-z0-9.-]{1,64}$`)

func directSMARTOriginFromForm(c *gin.Context) (*directSMARTOrigin, error) {
	// storeFileLocally has parsed the multipart form. Presence of an empty field
	// must be rejected, not mistaken for a legacy manual import.
	values, present := c.Request.MultipartForm.Value["smart_origin"]
	if _, fileField := c.Request.MultipartForm.File["smart_origin"]; fileField {
		return nil, fmt.Errorf("smart_origin must be one JSON form field")
	}
	if !present {
		return nil, nil
	}
	if len(values) != 1 || len(values[0]) > 4096 || !utf8.ValidString(values[0]) {
		return nil, fmt.Errorf("smart_origin must be one UTF-8 JSON object of at most 4096 bytes")
	}
	var origin directSMARTOrigin
	decoder := json.NewDecoder(strings.NewReader(values[0]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&origin); err != nil {
		return nil, fmt.Errorf("invalid smart_origin JSON")
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return nil, fmt.Errorf("smart_origin must contain only one JSON object")
	}
	origin.Display = strings.TrimSpace(origin.Display)
	if n := utf8.RuneCountInString(origin.Display); n < 1 || n > 100 {
		return nil, fmt.Errorf("smart_origin display must contain 1 to 100 characters")
	}
	base, err := url.Parse(origin.FHIRBaseURL)
	if err != nil || len(origin.FHIRBaseURL) > 2048 || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.Opaque != "" || base.RawQuery != "" || base.ForceQuery || strings.Contains(origin.FHIRBaseURL, "#") {
		return nil, fmt.Errorf("smart_origin fhir_base_url must be an absolute HTTPS URL without credentials, query or fragment, at most 2048 bytes")
	}
	origin.FHIRBaseURL = strings.TrimRight(base.String(), "/")
	return &origin, nil
}

func directSMARTPatientID(file *os.File) (string, error) {
	defer file.Seek(0, io.SeekStart)
	// The host bounds the fetched Patient to 2 MiB; allow room for its Bundle
	// wrapper without allowing arbitrary uploads to allocate unbounded memory.
	const maxSnapshotBytes = 3 * 1024 * 1024
	raw, err := io.ReadAll(io.LimitReader(file, maxSnapshotBytes+1))
	if err != nil {
		return "", fmt.Errorf("unable to read direct SMART snapshot")
	}
	if len(raw) > maxSnapshotBytes {
		return "", fmt.Errorf("direct SMART snapshot must not exceed 3 MiB")
	}
	var bundle struct {
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			Resource struct {
				ResourceType string `json:"resourceType"`
				ID           string `json:"id"`
			} `json:"resource"`
		} `json:"entry"`
	}
	if !utf8.Valid(raw) || json.Unmarshal(raw, &bundle) != nil || bundle.ResourceType != "Bundle" || len(bundle.Entry) != 1 || bundle.Entry[0].Resource.ResourceType != "Patient" || !directSMARTPatientIDPattern.MatchString(bundle.Entry[0].Resource.ID) {
		return "", fmt.Errorf("direct SMART snapshot must be a UTF-8 FHIR Bundle containing exactly one Patient with a valid FHIR ID")
	}
	return bundle.Entry[0].Resource.ID, nil
}

func directSMARTPatientExists(ctx context.Context, repo database.DatabaseRepository, sourceID, patientID string) (bool, error) {
	resources, err := repo.ListResources(ctx, models.ListResourceQueryOptions{SourceID: sourceID, SourceResourceType: "Patient", SourceResourceID: patientID, Limit: 1})
	return len(resources) > 0, err
}

func storeFileLocally(c *gin.Context) (*os.File, error) {
	// single file
	file, err := c.FormFile("file")
	if err != nil {
		//c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "could not extract file from form"})
		return nil, fmt.Errorf("could not extract file from form")
	}
	fmt.Printf("Uploaded filename: %s", file.Filename)

	// create a temporary file to store this uploaded file
	bundleFile, err := ioutil.TempFile("", file.Filename)
	if err != nil {
		//c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "could not create temp file"})
		return nil, fmt.Errorf("could not create temp file")
	}

	// Upload the file to specific bundleFile.
	err = c.SaveUploadedFile(file, bundleFile.Name())
	if err != nil {
		bundleFile.Close()
		os.Remove(bundleFile.Name())
		//c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "could not save temp file"})
		return nil, fmt.Errorf("could not save temp file")
	}
	return bundleFile, nil
}
