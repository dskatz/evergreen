package host

import (
	"github.com/evergreen-ci/evergreen/db"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

type Volume struct {
	ID               string `bson:"_id" json:"id"`
	CreatedBy        string `bson:"created_by" json:"created_by"`
	Type             string `bson:"type" json:"type"`
	Size             int    `bson:"size" json:"size"`
	AvailabilityZone string `bson:"availability_zone" json:"availability_zone"`
}

// Insert a volume into the volumes collection.
func (v *Volume) Insert() error {
	return db.Insert(VolumesCollection, v)
}

// Remove a volume from the volumes collection.
// Note this shouldn't be used when you want to
// remove from AWS itself.
func (v *Volume) Remove() error {
	return db.Remove(
		VolumesCollection,
		bson.M{
			IdKey: v.ID,
		},
	)
}

// FindVolumeByID finds a volume by its ID field.
func FindVolumeByID(id string) (*Volume, error) {
	return FindOneVolume(bson.M{VolumeIDKey: id})
}

type volumeSize struct {
	TotalVolumeSize int `bson:"total"`
}

func FindTotalVolumeSizeByUser(user string) (int, error) {
	pipeline := []bson.M{
		{"$match": bson.M{
			VolumeCreatedByKey: user,
		}},
		{"$group": bson.M{
			"_id":   "123",
			"total": bson.M{"$sum": "$" + VolumeSizeKey},
		}},
	}

	out := []volumeSize{}
	err := db.Aggregate(VolumesCollection, pipeline, &out)
	if err != nil || len(out) == 0 {
		return 0, err
	}

	return out[0].TotalVolumeSize, nil
}

func ValidateVolumeCanBeAttached(volumeID string) (*Volume, error) {
	volume, err := FindVolumeByID(volumeID)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get volume '%s'", volumeID)
	}
	if volume == nil {
		return nil, errors.Errorf("volume '%s' does not exist", volumeID)
	}
	var sourceHost *Host
	sourceHost, err = FindHostWithVolume(volumeID)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get source host for volume '%s'", volumeID)
	}
	if sourceHost != nil {
		return nil, errors.Errorf("volume '%s' is already attached to host '%s'", volumeID, sourceHost.Id)
	}
	return volume, nil
}
