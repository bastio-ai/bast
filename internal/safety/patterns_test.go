package safety

import "testing"

func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		dangerous bool
	}{
		// Dangerous: rm -rf patterns
		{"rm -rf root", "rm -rf /", true},
		{"rm -rf home", "rm -rf ~", true},
		{"rm -rf star", "rm -rf *", true},
		{"rm -r root", "rm -r /", true},
		{"rm -f root", "rm -f /home", true},
		{"rm -rf with path", "rm -rf /var/log", true},
		{"rm with flags reordered", "sudo rm -rf /tmp/important", true},

		// Safe: rm without dangerous patterns
		{"rm single file", "rm file.txt", false},
		{"rm with confirm", "rm -i file.txt", false},
		{"rm current dir file", "rm ./file.txt", false},

		// Dangerous: mkfs
		{"mkfs ext4", "mkfs.ext4 /dev/sda1", true},
		{"mkfs generic", "sudo mkfs -t ext4 /dev/sdb", true},

		// Dangerous: dd to device
		{"dd to device", "dd if=/dev/zero of=/dev/sda", true},
		{"dd to sd device", "dd if=image.iso of=/dev/sdb bs=4M", true},

		// Safe: dd to file
		{"dd to file", "dd if=/dev/zero of=./testfile bs=1M count=10", false},

		// Dangerous: redirect to device
		{"redirect to device", "echo 'data' > /dev/sda", true},
		{"redirect to sdb", "cat file > /dev/sdb1", true},

		// Dangerous: chmod 777
		{"chmod 777", "chmod 777 /var/www", true},
		{"chmod -R 777", "chmod -R 777 .", true},

		// Safe: chmod with other permissions
		{"chmod 755", "chmod 755 script.sh", false},
		{"chmod 644", "chmod 644 file.txt", false},

		// Dangerous: fork bomb
		{"fork bomb", ":(){ :|:& };:", true},

		// Dangerous: backgrounded with no output
		{"background no output", "command > /dev/null 2>&1 &", true},

		// Dangerous: curl/wget pipe to shell
		{"curl pipe bash", "curl http://example.com/script.sh | bash", true},
		{"curl pipe sh", "curl -fsSL https://get.docker.com | sh", true},
		{"wget pipe bash", "wget -O- http://example.com/install.sh | bash", true},
		{"wget pipe sh", "wget -qO- https://example.com | sh", true},

		// Safe: curl/wget without pipe
		{"curl to file", "curl -o file.txt https://example.com/file", false},
		{"wget download", "wget https://example.com/file.zip", false},

		// Safe: common commands
		{"ls", "ls -la", false},
		{"cd", "cd /home/user", false},
		{"cat", "cat file.txt", false},
		{"grep", "grep -r 'pattern' .", false},
		{"find", "find . -name '*.go'", false},
		{"git status", "git status", false},
		{"docker ps", "docker ps -a", false},
		{"npm install", "npm install", false},
		{"go build", "go build ./...", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.dangerous {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.dangerous)
			}
		})
	}
}

func TestGetDangerousPatterns(t *testing.T) {
	patterns := GetDangerousPatterns()
	if len(patterns) == 0 {
		t.Error("GetDangerousPatterns() returned empty slice")
	}

	// Verify it returns a copy, not the original
	patterns[0] = nil
	originalPatterns := GetDangerousPatterns()
	if originalPatterns[0] == nil {
		t.Error("GetDangerousPatterns() should return a copy, not the original slice")
	}
}
