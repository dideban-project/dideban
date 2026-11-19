package collector

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// mountsSample is a representative /proc/mounts for a Linux desktop. It is NOT
// captured from any host: it was written by hand to follow the /proc/mounts
// format, and its devices, paths and options are invented. It mixes the
// supported filesystems (ext4, xfs, btrfs) with virtual, temporary, network,
// container and other filesystems that must be skipped.
const mountsSample = `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
udev /dev devtmpfs rw,nosuid,relatime,size=8167648k,nr_inodes=2041912,mode=755 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=1637452k,mode=755 0 0
/dev/nvme0n1p2 / ext4 rw,relatime,errors=remount-ro 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
cgroup2 /sys/fs/cgroup cgroup2 rw,nosuid,nodev,noexec,relatime,nsdelegate 0 0
/dev/nvme0n1p1 /boot/efi vfat rw,relatime,fmask=0077,dmask=0077,errors=remount-ro 0 0
/dev/sda1 /data xfs rw,relatime,attr2,inode64,logbufs=8,logbsize=32k,noquota 0 0
/dev/sdb1 /home btrfs rw,relatime,ssd,space_cache=v2,subvolid=5,subvol=/ 0 0
/dev/loop0 /snap/core22/1380 squashfs ro,nodev,relatime,errors=continue 0 0
server:/export /mnt/nfs nfs4 rw,relatime,vers=4.2 0 0
overlay /var/lib/docker/overlay2/abc/merged overlay rw,relatime,lowerdir=/l1:/l2 0 0
`

// rwMount returns the Mount that the line "<device> <mountPoint> <fsType> rw 0 0" parses to.
func rwMount(device, mountPoint, fsType string) Mount {
	return Mount{Device: device, MountPoint: mountPoint, FSType: fsType, Options: "rw"}
}

func sameMounts(got, want []Mount) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestParseMountsSample(t *testing.T) {
	got, err := ParseMounts(strings.NewReader(mountsSample))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Mount{
		{Device: "/dev/nvme0n1p2", MountPoint: "/", FSType: "ext4", Options: "rw,relatime,errors=remount-ro"},
		{Device: "/dev/sda1", MountPoint: "/data", FSType: "xfs", Options: "rw,relatime,attr2,inode64,logbufs=8,logbsize=32k,noquota"},
		{Device: "/dev/sdb1", MountPoint: "/home", FSType: "btrfs", Options: "rw,relatime,ssd,space_cache=v2,subvolid=5,subvol=/"},
	}
	if !sameMounts(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseMountsFilesystemFiltering(t *testing.T) {
	supported := []string{"ext4", "xfs", "btrfs"}
	for _, fs := range supported {
		t.Run("included "+fs, func(t *testing.T) {
			got, err := ParseMounts(strings.NewReader("/dev/x /mnt/x " + fs + " rw 0 0\n"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := []Mount{rwMount("/dev/x", "/mnt/x", fs)}
			if !sameMounts(got, want) {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}

	skipped := []string{
		"tmpfs", "proc", "sysfs", "devtmpfs", "devpts", "cgroup", "cgroup2",
		"overlay", "squashfs", "securityfs", "debugfs", "tracefs", "mqueue",
		"nfs", "nfs4", "cifs", "fuse", "fuse.sshfs", "vfat", "ntfs", "iso9660",
		"ext2", "ext3", "zfs", "ramfs", "autofs", "binfmt_misc",
		// Only exact matches count: similar names must not slip through.
		"EXT4", "ext4dev", "xfs2", "btrfs-x", "ext",
	}
	for _, fs := range skipped {
		t.Run("skipped "+fs, func(t *testing.T) {
			got, err := ParseMounts(strings.NewReader("/dev/x /mnt/x " + fs + " rw 0 0\n"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("got %+v, want no mounts", got)
			}
		})
	}
}

func TestParseMounts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Mount
		wantErr bool
	}{
		{
			name:  "single ext4 mount",
			input: "/dev/sda1 / ext4 rw,relatime 0 0\n",
			want:  []Mount{{Device: "/dev/sda1", MountPoint: "/", FSType: "ext4", Options: "rw,relatime"}},
		},
		{
			name:  "dump and pass are parsed",
			input: "/dev/sda1 / ext4 rw 1 2\n",
			want:  []Mount{{Device: "/dev/sda1", MountPoint: "/", FSType: "ext4", Options: "rw", Dump: 1, Pass: 2}},
		},
		{
			name: "input order is kept",
			input: "/dev/sdb1 /b xfs rw 0 0\n" +
				"/dev/sda1 /a ext4 rw 0 0\n" +
				"/dev/sdc1 /c btrfs rw 0 0\n",
			want: []Mount{
				rwMount("/dev/sdb1", "/b", "xfs"),
				rwMount("/dev/sda1", "/a", "ext4"),
				rwMount("/dev/sdc1", "/c", "btrfs"),
			},
		},
		{
			name:  "no trailing newline",
			input: "/dev/sda1 / ext4 rw 0 0",
			want:  []Mount{rwMount("/dev/sda1", "/", "ext4")},
		},
		{
			name:  "blank lines and extra whitespace",
			input: "\n  /dev/sda1\t/data   ext4  rw 0 0  \n\n",
			want:  []Mount{rwMount("/dev/sda1", "/data", "ext4")},
		},
		{
			name:  "space in mount point is decoded",
			input: `/dev/sda1 /mnt/my\040disk ext4 rw 0 0` + "\n",
			want:  []Mount{rwMount("/dev/sda1", "/mnt/my disk", "ext4")},
		},
		{
			name:  "tab and backslash in mount point are decoded",
			input: `/dev/sda1 /mnt/a\011b\134c ext4 rw 0 0` + "\n",
			want:  []Mount{rwMount("/dev/sda1", "/mnt/a\tb\\c", "ext4")},
		},
		{
			name:  "newline in mount point is decoded",
			input: `/dev/sda1 /mnt/a\012b ext4 rw 0 0` + "\n",
			want:  []Mount{rwMount("/dev/sda1", "/mnt/a\nb", "ext4")},
		},
		{
			name:  "space in device is decoded",
			input: `/dev/disk/by-label/My\040Disk /data ext4 rw 0 0` + "\n",
			want:  []Mount{rwMount("/dev/disk/by-label/My Disk", "/data", "ext4")},
		},
		{
			name:  "space in options is decoded",
			input: `/dev/sda1 /data ext4 rw,label=a\040b 0 0` + "\n",
			want:  []Mount{{Device: "/dev/sda1", MountPoint: "/data", FSType: "ext4", Options: "rw,label=a b"}},
		},
		{
			name:  "invalid escape sequences are kept",
			input: `/dev/sda1 /mnt/a\04 ext4 rw 0 0` + "\n" + `/dev/sdb1 /mnt/b\999 ext4 rw 0 0` + "\n",
			want: []Mount{
				rwMount("/dev/sda1", `/mnt/a\04`, "ext4"),
				rwMount("/dev/sdb1", `/mnt/b\999`, "ext4"),
			},
		},
		{
			name: "same mount point twice keeps the last entry in the first position",
			input: "/dev/sda1 /data ext4 rw 0 0\n" +
				"/dev/sdb1 /other xfs rw 0 0\n" +
				"/dev/sdc1 /data btrfs rw 0 0\n",
			want: []Mount{
				rwMount("/dev/sdc1", "/data", "btrfs"),
				rwMount("/dev/sdb1", "/other", "xfs"),
			},
		},
		{
			name: "same device on two mount points is kept twice",
			input: "/dev/sda1 /a ext4 rw 0 0\n" +
				"/dev/sda1 /b ext4 rw 0 0\n",
			want: []Mount{
				rwMount("/dev/sda1", "/a", "ext4"),
				rwMount("/dev/sda1", "/b", "ext4"),
			},
		},
		{
			name:  "only unsupported filesystems",
			input: "proc /proc proc rw 0 0\ntmpfs /run tmpfs rw 0 0\n",
			want:  nil,
		},
		{
			name:  "unsupported entry with three fields is ignored",
			input: "tmpfs /run tmpfs\n",
			want:  nil,
		},
		{
			name:  "unsupported entry with too few fields is ignored",
			input: "proc /proc proc rw\n",
			want:  nil,
		},
		{
			name:  "unsupported entry with too many fields is ignored",
			input: "proc /proc proc rw 0 0 extra\n",
			want:  nil,
		},
		{
			name:  "unsupported entry with invalid dump and pass is ignored",
			input: "proc /proc proc rw x y\n",
			want:  nil,
		},
		{name: "empty input", input: "", wantErr: true},
		{name: "blank lines only", input: "\n  \n\t\n", wantErr: true},
		{name: "single field", input: "/dev/sda1\n", wantErr: true},
		{name: "two fields", input: "/dev/sda1 /\n", wantErr: true},
		{name: "malformed line among valid lines", input: "/dev/sda1 / ext4 rw 0 0\nbroken\n/dev/sdb1 /b xfs rw 0 0\n", wantErr: true},
		{name: "supported entry with three fields", input: "/dev/sda1 /data ext4\n", wantErr: true},
		{name: "supported entry with four fields", input: "/dev/sda1 /data ext4 rw\n", wantErr: true},
		{name: "supported entry with five fields", input: "/dev/sda1 /data ext4 rw 0\n", wantErr: true},
		{name: "supported entry with seven fields", input: "/dev/sda1 /data ext4 rw 0 0 extra\n", wantErr: true},
		{name: "non-numeric dump", input: "/dev/sda1 /data xfs rw x 0\n", wantErr: true},
		{name: "non-numeric pass", input: "/dev/sda1 /data btrfs rw 0 y\n", wantErr: true},
		{name: "negative dump", input: "/dev/sda1 /data ext4 rw -1 0\n", wantErr: true},
		{name: "negative pass", input: "/dev/sda1 /data ext4 rw 0 -1\n", wantErr: true},
		{name: "pass out of range", input: "/dev/sda1 /data ext4 rw 0 99999999999\n", wantErr: true},
		{name: "decimal dump", input: "/dev/sda1 /data ext4 rw 1.5 0\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMounts(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !sameMounts(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseMountsMalformedLineReportsLineNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "line with too few fields",
			input: "/dev/sda1 / ext4 rw 0 0\n" +
				"proc /proc proc rw 0 0\n" +
				"broken\n",
		},
		{
			name: "supported entry with a bad field count",
			input: "/dev/sda1 / ext4 rw 0 0\n" +
				"proc /proc proc rw 0 0\n" +
				"/dev/sdb1 /b xfs rw\n",
		},
		{
			name: "supported entry with a bad number",
			input: "/dev/sda1 / ext4 rw 0 0\n" +
				"proc /proc proc rw 0 0\n" +
				"/dev/sdb1 /b xfs rw x 0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMounts(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("expected error, got %+v", got)
			}
			if got != nil {
				t.Errorf("got %+v on error, want nil", got)
			}
			if !strings.Contains(err.Error(), "line 3") {
				t.Errorf("error %q does not mention the offending line number", err)
			}
		})
	}
}

func TestUnescapeMountField(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/plain/path", "/plain/path"},
		{`/a\040b`, "/a b"},
		{`/a\011b`, "/a\tb"},
		{`/a\012b`, "/a\nb"},
		{`/a\134b`, `/a\b`},
		{`\040`, " "},
		{`/a\040\040b`, "/a  b"},
		{`/a\04`, `/a\04`},
		{`/a\0`, `/a\0`},
		{`/a\`, `/a\`},
		{`/a\089`, `/a\089`},
		{`/a\400`, `/a\400`},
		{`/a\xyz`, `/a\xyz`},
		{"", ""},
	}
	for _, tt := range tests {
		if got := unescapeMountField(tt.in); got != tt.want {
			t.Errorf("unescapeMountField(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseMountsReaderError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := ParseMounts(iotest.ErrReader(sentinel))
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the underlying reader error", err)
	}
}

func TestParseMountsReaderErrorAfterValidData(t *testing.T) {
	sentinel := errors.New("read failed after data")
	r := io.MultiReader(strings.NewReader(mountsSample), iotest.ErrReader(sentinel))

	got, err := ParseMounts(r)
	if err == nil {
		t.Fatalf("expected reader error, got %+v and nil error", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the underlying reader error", err)
	}
	if got != nil {
		t.Errorf("got %+v on error, want nil", got)
	}
}

func TestParseMountsLineTooLong(t *testing.T) {
	// A line longer than bufio.Scanner's limit must surface as an error.
	input := "/dev/sda1 /" + strings.Repeat("a", 70*1024) + " ext4 rw 0 0\n"

	got, err := ParseMounts(strings.NewReader(input))
	if err == nil {
		t.Fatalf("expected error for an over-long line, got %+v", got)
	}
	if got != nil {
		t.Errorf("got %+v on error, want nil", got)
	}
}
