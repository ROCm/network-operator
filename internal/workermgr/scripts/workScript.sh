#!/bin/bash

# Check and remove AMD GPU kernel modules
check_and_remove_amdgpu() {
    echo "Checking for AMD GPU kernel modules..."
    
    # Check if amdgpu module is loaded
    if lsmod | grep -q "amdgpu"; then
        echo "AMD GPU modules detected. Attempting to remove..."
        
        # List of AMD modules to remove in dependency order
        # Remove dependent modules first, then amdgpu last
        local modules=(
            "amdgpu"
            "amddrm_ttm_helper"
            "amdttm" 
            "amddrm_buddy"
            "amdxcp"
            "amddrm_exec"
            "amd_sched"
            "amdkcl"
        )
        
        # Try to remove each module
        for module in "${modules[@]}"; do
            # Only remove module if it is loaded
            if lsmod | grep -q "^${module}"; then
                echo "Removing module: $module"
                if ! rmmod "$module" 2>/dev/null; then
                    echo "Warning: Failed to remove $module (may be in use)"
                else
                    echo "Successfully removed $module"
                fi
            fi
        done
        
        # Final check
        if lsmod | grep -q "amdgpu"; then
            echo "ERROR: Some AMD GPU modules are still loaded"
            echo "Currently loaded AMD modules:"
            lsmod | grep -E "(amd|drm)" | sort
            return 1
        else
            echo "SUCCESS: All AMD GPU modules have been removed"
            return 0
        fi
    else
        echo "No AMD GPU modules currently loaded"
        return 0
    fi
}

# Run the function
check_and_remove_amdgpu
exit $?