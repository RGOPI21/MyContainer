#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <syscall.h>
#include <linux/sched.h>

//int clone (int (*fn) (void *), void *child_stack, int flags, void *arg,...)
static int childFunc(void *arg)
{
    printf("Child process (PID %d) inside new namespaces.\n", getpid());
    sleep(1);

    // casting the generic arg to a readable string pointer
    char *json_str = (char *)arg;
    char rootfs_path[1024] = {0};

    // Extract rootfs path string from JSON buffer
    char *target = strstr(json_str, "\"rootfs\":");
    if (target != NULL) {
        sscanf(target, "\"rootfs\": \"%[^\"]\"", rootfs_path);
    } else {
        fprintf(stderr, "Runtime Error: Failed to find rootfs key in config.json\n");
        return 1;
    }

    // Unsharing the mount namespace from the host system entirely to satisft linux's strict requirements 
    // for pivot_root
    if (mount("none", "/", NULL, MS_REC | MS_PRIVATE, NULL) < 0) {
        perror("Failed to make / private");
        return 1;
    }

    // 1. Turn rootfs into a mount point and create .oldroot 
    // Bind mount the dir to itself so linux recognize it as a filesystem boundary.
    if (mount(rootfs_path, rootfs_path, NULL, MS_BIND | MS_REC, NULL) < 0) {
        perror("Failed to bind mount rootfs.");
        return 1;
    }

    // 2. Create the temp old storage folder inside rootfs path
    char old_root_path[2048];
    snprintf(old_root_path, sizeof(old_root_path), "%s/.oldroot", rootfs_path);
    mkdir(old_root_path, 0777);

    // 3.Pivot the filesystem.
    /* Change working dir to new rootfs before swap */
    if (chdir(rootfs_path) < 0) {
        perror("chdir to rootfs failed");
        return 1;
    }

    // executing the actual pivot syscall swap.
    if (syscall(SYS_pivot_root, ".", ".oldroot") <0 ){
        perror("pivot_root system call failed");
        return 1;
    }

    // Re-anchoring and detach the old host filesystem
    /* moving to the global root path */
    if (chdir("/") < 0) {
        perror("chdir to global root failed");
        return 1;
    }
        
    // lazily unmount the host file system so that the containter cannot see or access it.
    if (umount2("/.oldroot", MNT_DETACH) < 0) {
        perror("Failed to unmount old rootfs");
        return 1;
    }

    // clean up: deleting the temp direct handle
    rmdir("/.oldroot");

    // 4. Implementing PseudoFS mounts (/proc) and (/sys)
    if (mkdir("/proc", 0755) < 0) {
        perror("Failed to create /proc directory.");
        return 1;
    }

    if (mkdir("/sys", 0755) < 0) {
        perror("Failed to create /sys directory.");
        return 1;
    }

    // 5. Mounting the /proc filesystem.
    if (mount("proc", "/proc", "proc", 0, NULL) < 0) {
        perror("Failed to mount /proc fs.");
        return 1;
    }

    // 6. Mounting the /sysfs filesystem.
    if (mount("sysfs", "/sys", "sysfs", 0, NULL) < 0) {
        perror("Failed to mount /sysfs.");
        return 1;
    }
    
    char *const argv[] = {"/bin/bash", NULL};
    char *const envp[] = {"PATH=/bin:/usr/bin", NULL};

    execve("/bin/bash", argv, envp);
    
    perror("execve failed");
    return 1;
    
}

int main(int argc, char *argv[])
{
    if(argc < 2) {
        fprintf(stderr, "Less number of arguments.\n");
        exit(1);
    }

    char *bundle_path = argv[1];
    char config_path[1024];
    snprintf(config_path, sizeof(config_path), "%s/config.json", bundle_path);

    FILE *targetFile = fopen(config_path, "rb");
    if (targetFile == NULL) {
        perror("Error opening file");
        return 1;
    }

    char buffer[1024];
    size_t read_buf = fread(buffer, sizeof(char),sizeof(buffer)-1, targetFile);
    buffer[read_buf] = '\0';

    fclose(targetFile);

    pid_t child_pid;
    int status; 

    struct clone_args args = {0};

    args.flags = CLONE_NEWCGROUP | CLONE_NEWIPC | CLONE_NEWNET | CLONE_NEWNS | CLONE_NEWPID | CLONE_NEWUTS;
    args.exit_signal = SIGCHLD; 
    
    child_pid = syscall(__NR_clone3, &args, sizeof(args));

    if (child_pid< 0) {
        perror("Clone3 failed"); 
        return 1;
    }

    if (child_pid == 0) {
        childFunc(buffer);
        _exit(0);
    } 

    if (waitpid(child_pid, &status, 0) == -1) {
       perror("waitpid failed"); 
    }

    return 0;
}
