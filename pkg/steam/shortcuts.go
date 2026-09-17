package steam

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strconv"
	"strings"
)

const (
	typeSubDict = 0x00
	typeString  = 0x01
	typeInt32   = 0x02
	typeEndDict = 0x08
)

// Shortcut represents a non-Steam game shortcut in shortcuts.vdf
type Shortcut struct {
	AppID               uint32            `json:"appid"`
	AppName             string            `json:"appname"`
	Exe                 string            `json:"exe"`
	StartDir            string            `json:"startDir"`
	Icon                string            `json:"icon"`
	ShortcutPath        string            `json:"shortcutPath"`
	LaunchOptions       string            `json:"launchOptions"`
	IsHidden            uint32            `json:"isHidden"`
	AllowDesktopConfig  uint32            `json:"allowDesktopConfig"`
	AllowOverlay        uint32            `json:"allowOverlay"`
	OpenVR              uint32            `json:"openvr"`
	Devkit              uint32            `json:"devkit"`
	DevkitGameID        string            `json:"devkitGameId"`
	DevkitOverrideAppID uint32            `json:"devkitOverrideAppId"`
	LastPlayTime        uint32            `json:"lastPlayTime"`
	FlatpakAppID        string            `json:"flatpakAppId"`
	Tags                []string          `json:"tags"`
	ExtraStrings        map[string]string `json:"extraStrings,omitempty"`
	ExtraInts           map[string]uint32 `json:"extraInts,omitempty"`
}

// GenerateAppID generates a deterministic 32-bit AppID for a shortcut based on exe path and app name
func GenerateAppID(exePath, appName string) uint32 {
	key := exePath + appName
	crc := crc32.ChecksumIEEE([]byte(key))
	return crc | 0x80000000
}

// ParseShortcuts parses a binary shortcuts.vdf stream
func ParseShortcuts(r io.Reader) ([]Shortcut, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	buf := bytes.NewReader(data)

	// Read top-level type and key (should be typeSubDict and "shortcuts")
	b, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != typeSubDict {
		return nil, fmt.Errorf("invalid binary vdf: expected 0x00 root, got 0x%02x", b)
	}

	rootName, err := readNullTerminatedString(buf)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(rootName, "shortcuts") {
		return nil, fmt.Errorf("unexpected root dictionary name: %s", rootName)
	}

	var shortcuts []Shortcut

	for {
		itemType, err := buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if itemType == typeEndDict {
			// End of shortcuts dictionary
			break
		}

		// Read index name (e.g. "0", "1", "2")
		_, err = readNullTerminatedString(buf)
		if err != nil {
			return nil, err
		}

		s := Shortcut{
			AllowDesktopConfig: 1,
			AllowOverlay:       1,
			ExtraStrings:       make(map[string]string),
			ExtraInts:          make(map[string]uint32),
		}

		// Read shortcut properties until typeEndDict
		for {
			propType, err := buf.ReadByte()
			if err != nil {
				return nil, err
			}
			if propType == typeEndDict {
				break
			}

			key, err := readNullTerminatedString(buf)
			if err != nil {
				return nil, err
			}

			keyLower := strings.ToLower(key)

			switch propType {
			case typeString:
				val, err := readNullTerminatedString(buf)
				if err != nil {
					return nil, err
				}
				switch keyLower {
				case "appname":
					s.AppName = val
				case "exe":
					s.Exe = val
				case "startdir":
					s.StartDir = val
				case "icon":
					s.Icon = val
				case "shortcutpath":
					s.ShortcutPath = val
				case "launchoptions":
					s.LaunchOptions = val
				case "devkitgameid":
					s.DevkitGameID = val
				case "flatpakappid":
					s.FlatpakAppID = val
				default:
					s.ExtraStrings[key] = val
				}

			case typeInt32:
				var val uint32
				if err := binary.Read(buf, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				switch keyLower {
				case "appid":
					s.AppID = val
				case "ishidden":
					s.IsHidden = val
				case "allowdesktopconfig":
					s.AllowDesktopConfig = val
				case "allowoverlay":
					s.AllowOverlay = val
				case "openvr":
					s.OpenVR = val
				case "devkit":
					s.Devkit = val
				case "devkitoverrideappid":
					s.DevkitOverrideAppID = val
				case "lastplaytime":
					s.LastPlayTime = val
				default:
					s.ExtraInts[key] = val
				}

			case typeSubDict:
				if keyLower == "tags" {
					tags, err := parseTagsDict(buf)
					if err != nil {
						return nil, err
					}
					s.Tags = tags
				} else {
					// Skip unknown sub-dict
					if err := skipSubDict(buf); err != nil {
						return nil, err
					}
				}

			default:
				return nil, fmt.Errorf("unknown property type 0x%02x for key %s", propType, key)
			}
		}

		shortcuts = append(shortcuts, s)
	}

	return shortcuts, nil
}

func parseTagsDict(buf *bytes.Reader) ([]string, error) {
	var tags []string
	for {
		t, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if t == typeEndDict {
			break
		}
		// Read index name (e.g. "0")
		_, err = readNullTerminatedString(buf)
		if err != nil {
			return nil, err
		}
		tagVal, err := readNullTerminatedString(buf)
		if err != nil {
			return nil, err
		}
		if tagVal != "" {
			tags = append(tags, tagVal)
		}
	}
	return tags, nil
}

func skipSubDict(buf *bytes.Reader) error {
	for {
		t, err := buf.ReadByte()
		if err != nil {
			return err
		}
		if t == typeEndDict {
			return nil
		}
		_, err = readNullTerminatedString(buf)
		if err != nil {
			return err
		}
		switch t {
		case typeString:
			_, err = readNullTerminatedString(buf)
			if err != nil {
				return err
			}
		case typeInt32:
			var dummy uint32
			if err := binary.Read(buf, binary.LittleEndian, &dummy); err != nil {
				return err
			}
		case typeSubDict:
			if err := skipSubDict(buf); err != nil {
				return err
			}
		}
	}
}

func readNullTerminatedString(r *bytes.Reader) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			break
		}
		sb.WriteByte(b)
	}
	return sb.String(), nil
}

// WriteShortcuts serializes a list of shortcuts into the binary VDF format
func WriteShortcuts(w io.Writer, shortcuts []Shortcut) error {
	buf := new(bytes.Buffer)

	// Root dict: \x00shortcuts\x00
	buf.WriteByte(typeSubDict)
	buf.WriteString("shortcuts")
	buf.WriteByte(0)

	for i, s := range shortcuts {
		// Entry dict: \x00<i>\x00
		buf.WriteByte(typeSubDict)
		buf.WriteString(strconv.Itoa(i))
		buf.WriteByte(0)

		// 1. appid
		writeUInt32Prop(buf, "appid", s.AppID)

		// 2. appname
		writeStringProp(buf, "appname", s.AppName)

		// 3. exe
		writeStringProp(buf, "exe", s.Exe)

		// 4. StartDir
		writeStringProp(buf, "StartDir", s.StartDir)

		// 5. icon
		writeStringProp(buf, "icon", s.Icon)

		// 6. ShortcutPath
		writeStringProp(buf, "ShortcutPath", s.ShortcutPath)

		// 7. LaunchOptions
		writeStringProp(buf, "LaunchOptions", s.LaunchOptions)

		// 8. IsHidden
		writeUInt32Prop(buf, "IsHidden", s.IsHidden)

		// 9. AllowDesktopConfig
		writeUInt32Prop(buf, "AllowDesktopConfig", s.AllowDesktopConfig)

		// 10. AllowOverlay
		writeUInt32Prop(buf, "AllowOverlay", s.AllowOverlay)

		// 11. openvr
		writeUInt32Prop(buf, "openvr", s.OpenVR)

		// 12. Devkit
		writeUInt32Prop(buf, "Devkit", s.Devkit)

		// 13. DevkitGameID
		writeStringProp(buf, "DevkitGameID", s.DevkitGameID)

		// 14. DevkitOverrideAppID
		writeUInt32Prop(buf, "DevkitOverrideAppID", s.DevkitOverrideAppID)

		// 15. LastPlayTime
		writeUInt32Prop(buf, "LastPlayTime", s.LastPlayTime)

		// 16. FlatpakAppID
		writeStringProp(buf, "FlatpakAppID", s.FlatpakAppID)

		// Extra strings preserved
		for k, v := range s.ExtraStrings {
			writeStringProp(buf, k, v)
		}

		// Extra ints preserved
		for k, v := range s.ExtraInts {
			writeUInt32Prop(buf, k, v)
		}

		// Tags sub-dict
		buf.WriteByte(typeSubDict)
		buf.WriteString("tags")
		buf.WriteByte(0)
		for ti, tag := range s.Tags {
			writeStringProp(buf, strconv.Itoa(ti), tag)
		}
		buf.WriteByte(typeEndDict)

		// End of shortcut entry
		buf.WriteByte(typeEndDict)
	}

	// End of shortcuts dict
	buf.WriteByte(typeEndDict)
	// End of root dict
	buf.WriteByte(typeEndDict)

	_, err := w.Write(buf.Bytes())
	return err
}

func writeStringProp(buf *bytes.Buffer, key, val string) {
	buf.WriteByte(typeString)
	buf.WriteString(key)
	buf.WriteByte(0)
	buf.WriteString(val)
	buf.WriteByte(0)
}

func writeUInt32Prop(buf *bytes.Buffer, key string, val uint32) {
	buf.WriteByte(typeInt32)
	buf.WriteString(key)
	buf.WriteByte(0)
	_ = binary.Write(buf, binary.LittleEndian, val)
}
