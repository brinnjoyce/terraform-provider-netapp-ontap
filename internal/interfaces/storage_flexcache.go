package interfaces

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/mitchellh/mapstructure"
	"github.com/netapp/terraform-provider-netapp-ontap/internal/restclient"
	"github.com/netapp/terraform-provider-netapp-ontap/internal/utils"
)

// StorageFlexcacheGetDataModelONTAP describes the GET record data model using go types for mapping.
type StorageFlexcacheGetDataModelONTAP struct {
	Name                     string
	SVM                      svm
	Aggregates               []StorageFlexcacheAggregate `mapstructure:"aggregates"`
	Origins                  []StorageFlexcacheOrigin    `mapstructure:"origins"`
	JunctionPath             string                      `mapstructure:"junction_path,omitempty"`
	Size                     int                         `mapstructure:"size,omitempty"`
	Path                     string                      `mapstructure:"path,omitempty"`
	Guarantee                StorageFlexcacheGuarantee   `mapstructure:"guarantee,omitempty"`
	DrCache                  bool                        `mapstructure:"dr_cache,omitempty"`
	GlobalFileLockingEnabled bool                        `mapstructure:"global_file_locking_enabled,omitempty"`
	UseTieredAggregate       bool                        `mapstructure:"use_tiered_aggregate,omitempty"`
	ConstituentsPerAggregate int                         `mapstructure:"constituents_per_aggregate,omitempty"`
	UUID                     string
}

// StorageFlexcacheResourceModel describes the resource data model.
type StorageFlexcacheResourceModel struct {
	Name                     string                    `mapstructure:"name,omitempty"`
	SVM                      svm                       `mapstructure:"svm,omitempty"`
	Origins                  []map[string]interface{}  `mapstructure:"origins,omitempty"`
	JunctionPath             string                    `mapstructure:"junction_path,omitempty"`
	Size                     int                       `mapstructure:"size,omitempty"`
	Path                     string                    `mapstructure:"path,omitempty"`
	Guarantee                StorageFlexcacheGuarantee `mapstructure:"guarantee,omitempty"`
	DrCache                  bool                      `mapstructure:"dr_cache"`
	GlobalFileLockingEnabled bool                      `mapstructure:"global_file_locking_enabled"`
	UseTieredAggregate       bool                      `mapstructure:"use_tiered_aggregate"`
	ConstituentsPerAggregate int                       `mapstructure:"constituents_per_aggregate,omitempty"`
	Aggregates               []map[string]interface{}  `mapstructure:"aggregates,omitempty"`
}

// StorageFlexcacheGuarantee describes the guarantee data model of Guarantee within StorageFlexcacheResourceModel.
type StorageFlexcacheGuarantee struct {
	Type string `mapstructure:"type,omitempty"`
}

// StorageFlexcacheOrigin describes the origin data model of Origin within StorageFlexcacheResourceModel.
type StorageFlexcacheOrigin struct {
	Volume StorageFlexcacheVolume `mapstructure:"volume"`
	SVM    StorageFlexcacheSVM    `mapstructure:"svm"`
}

// StorageFlexcacheVolume describes the volume data model of Volume within StorageFlexcacheOrigin.
type StorageFlexcacheVolume struct {
	Name string `mapstructure:"name,omitempty"`
	ID   string `mapstructure:"uuid,omitempty"`
}

// StorageFlexcacheSVM describes the svm data model of SVM within StorageFlexcacheOrigin.
type StorageFlexcacheSVM struct {
	Name string `mapstructure:"name,omitempty"`
	ID   string `mapstructure:"uuid,omitempty"`
}

// StorageFlexcacheAggregate describes the aggregate data model of Aggregate within StorageFlexcacheResourceModel.
type StorageFlexcacheAggregate struct {
	Name string `mapstructure:"name,omitempty"`
	ID   string `mapstructure:"uuid,omitempty"`
}

// StorageFlexcacheDataSourceFilterModel describes the data source data model for queries.
type StorageFlexcacheDataSourceFilterModel struct {
	Name    string `mapstructure:"name"`
	SVMName string `mapstructure:"svm.name"`
}

// GetStorageFlexcacheByName to get flexcache info by name.
func GetStorageFlexcacheByName(errorHandler *utils.ErrorHandler, r restclient.RestClient, name string, svmName string) (*StorageFlexcacheGetDataModelONTAP, error) {
	// API responses are unreliable when trying to fetch by filter
	// Find the ID first and then get the extra fields
	query := r.NewQuery()
	query.Add("name", name)
	query.Add("svm.name", svmName)
	statusCode, response, err := r.GetNilOrOneRecord("storage/flexcache/flexcaches", query, nil)
	if err != nil {
		return nil, errorHandler.MakeAndReportError("error reading flexcache info", fmt.Sprintf("error on GET storage/flexcache/flexcaches: %s", err))
	}
	if response == nil {
		// No record found
		tflog.Debug(errorHandler.Ctx, fmt.Sprintf("No flexcache found for %s/%s", name, svmName))
		return nil, nil
	}

	// Extract href from the response
	var basic struct {
		Links struct {
			Self struct {
				Href string `mapstructure:"href"`
			} `mapstructure:"self"`
		} `mapstructure:"_links"`
	}
	if err := mapstructure.Decode(response, &basic); err != nil {
		return nil, errorHandler.MakeAndReportError("error decoding basic flexcache response", fmt.Sprintf("decode error: %v, statusCode %d, response %#v", err, statusCode, response))
	}
	href := strings.TrimPrefix(basic.Links.Self.Href, "/api/")
	if href == "" {
		return nil, errorHandler.MakeAndReportError("error reading flexcache info", fmt.Sprintf("missing href in basic flexcache response: %#v", response))
	}
	// Query the specific flexcache with the extra fields
	detailQuery := r.NewQuery()
	detailQuery.Fields([]string{"size", "path", "origins", "guarantee.type", "constituents_per_aggregate", "dr_cache", "global_file_locking_enabled", "aggregates"})

	statusCode, detailResponse, err := r.GetResponse(href, detailQuery, nil)
	if err != nil {
		return nil, errorHandler.MakeAndReportError(
			"error reading detailed flexcache info",
			fmt.Sprintf("GET %s failed: %v", href, err),
		)
	}

	var dataONTAP StorageFlexcacheGetDataModelONTAP
	if err := mapstructure.Decode(detailResponse, &dataONTAP); err != nil {
		return nil, errorHandler.MakeAndReportError("error decoding detailed flexcache info", fmt.Sprintf("decode error: %v, statusCode %d, response %#v", err, statusCode, detailResponse))
	}

	tflog.Debug(errorHandler.Ctx, fmt.Sprintf("Read flexcache source (detailed): %#v", dataONTAP))
	return &dataONTAP, nil
}

// GetStorageFlexcaches retrieves all FlexCache volumes that match the filter.
// It performs a lightweight list query first, then fetches full details per record
// to avoid ONTAP API issues with large field lists.
func GetStorageFlexcaches(errorHandler *utils.ErrorHandler, r restclient.RestClient, filter *StorageFlexcacheDataSourceFilterModel) ([]StorageFlexcacheGetDataModelONTAP, error) {
	api := "storage/flexcache/flexcaches"
	// Step 1: initial lightweight query (minimal fields)
	query := r.NewQuery()
	query.Fields([]string{"uuid", "name", "svm.name"})

	if filter != nil {
		var filterMap map[string]interface{}
		if err := mapstructure.Decode(filter, &filterMap); err != nil {
			return nil, errorHandler.MakeAndReportError(
				"error encoding storage flexcache filter info",
				fmt.Sprintf("error on filter %#v: %s", filter, err),
			)
		}
		query.SetValues(filterMap)
	}

	// Fetch basic list of flexcache records
	statusCode, response, err := r.GetZeroOrMoreRecords(api, query, nil)
	if err != nil {
		return nil, errorHandler.MakeAndReportError(
			"error reading storage flexcache list",
			fmt.Sprintf("GET %s failed: %s, statusCode %d", api, err, statusCode),
		)
	}

	if response == nil || len(response) == 0 {
		tflog.Debug(errorHandler.Ctx, fmt.Sprintf("No flexcache records found for %s", api))
		return []StorageFlexcacheGetDataModelONTAP{}, nil
	}

	// Step 2: iterate over each record and fetch full details
	var results []StorageFlexcacheGetDataModelONTAP

	for _, record := range response {
		var basic struct {
			Links struct {
				Self struct {
					Href string `mapstructure:"href"`
				} `mapstructure:"self"`
			} `mapstructure:"_links"`
		}

		if err := mapstructure.Decode(record, &basic); err != nil {
			errorHandler.MakeAndReportError(
				"error decoding flexcache href info",
				fmt.Sprintf("decode error: %v, record: %#v", err, record),
			)
			continue
		}

		href := strings.TrimPrefix(basic.Links.Self.Href, "/api/")
		if href == "" {
			errorHandler.MakeAndReportError(
				"missing href in flexcache record",
				fmt.Sprintf("record: %#v", record),
			)
			continue
		}

		// Query with detailed fields for each record
		detailQuery := r.NewQuery()
		detailQuery.Fields([]string{
			"size",
			"path",
			"origins",
			"guarantee.type",
			"constituents_per_aggregate",
			"dr_cache",
			"global_file_locking_enabled",
			"aggregates",
		})

		statusCode, detailResponse, err := r.GetResponse(href, detailQuery, nil)
		if err != nil {
			errorHandler.MakeAndReportError(
				"error fetching detailed flexcache info",
				fmt.Sprintf("GET %s failed: %v", href, err),
			)
			continue
		}

		var detailedRecord StorageFlexcacheGetDataModelONTAP
		if err := mapstructure.Decode(detailResponse, &detailedRecord); err != nil {
			errorHandler.MakeAndReportError(
				"error decoding detailed flexcache info",
				fmt.Sprintf("decode error: %v, statusCode %d, response %#v", err, statusCode, detailResponse),
			)
			continue
		}

		results = append(results, detailedRecord)
	}

	tflog.Debug(errorHandler.Ctx, fmt.Sprintf("Fetched %d flexcaches: %#v", len(results), results))
	return results, nil
}

// CreateStorageFlexcache creates flexcache.
// POST API returns result, but does not include the attributes that are not set. Make a speparate GET call to get all attributes.
func CreateStorageFlexcache(errorHandler *utils.ErrorHandler, r restclient.RestClient, data StorageFlexcacheResourceModel) error {
	var body map[string]interface{}
	if err := mapstructure.Decode(data, &body); err != nil {
		return errorHandler.MakeAndReportError("error encoding flexcache body", fmt.Sprintf("error on encoding storage/flexcache/flexcaches body: %s, body: %#v", err, data))
	}
	//The use-tiered-aggregate option is only supported when auto provisioning the FlexCache volume
	if _, ok := body["aggregates"]; ok {
		delete(body, "use_tiered_aggregate")
	}
	query := r.NewQuery()
	query.Add("return_records", "false")
	statusCode, _, err := r.CallCreateMethod("storage/flexcache/flexcaches", query, body)
	if err != nil {
		return errorHandler.MakeAndReportError("error creating flexcache", fmt.Sprintf("error on POST storage/flexcache/flexcaches: %s, statusCode %d", err, statusCode))
	}

	return nil

}

// DeleteStorageFlexcache to delete flexcache by id.
func DeleteStorageFlexcache(errorHandler *utils.ErrorHandler, r restclient.RestClient, id string) error {
	statusCode, _, err := r.CallDeleteMethod("storage/flexcache/flexcaches/"+id, nil, nil)
	if err != nil {
		return errorHandler.MakeAndReportError("error deleting flexcache", fmt.Sprintf("error on DELETE storage/flexcache/flexcaches: %s, statusCode %d", err, statusCode))
	}
	return nil
}
