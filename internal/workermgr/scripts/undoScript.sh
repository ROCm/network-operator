#!/bin/bash

# Check and load AMD GPU kernel modules
check_and_load_amdgpu() {
    echo "Checking for AMD GPU kernel modules..."
    
    # Check if amdgpu module exists
    if modinfo amdgpu &>/dev/null; then
        echo "AMD GPU module found in system"
        
        # Check if amdgpu module is already loaded
        if lsmod | grep -q "^amdgpu"; then
            echo "AMD GPU module is already loaded"
            return 0
        fi
        
        echo "Attempting to load AMD GPU module..."
        if modprobe amdgpu; then
            echo "SUCCESS: AMD GPU module loaded successfully"
            
            # Show loaded AMD modules
            echo "Currently loaded AMD modules:"
            lsmod | grep -E "(amd|drm)" | sort
            return 0
        else
            echo "ERROR: Failed to load AMD GPU module"
            return 1
        fi
    else
        echo "AMD GPU module not found in system"
        return 0
    fi
}

# Run the function
check_and_load_amdgpu
exit $?
