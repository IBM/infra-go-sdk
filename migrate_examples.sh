#!/bin/bash
# migrate_examples.sh - Automated script to update example files with context.Context

set -e

echo "==================================="
echo "Context Migration Script"
echo "==================================="
echo ""

# Counter for tracking changes
total_files=0
updated_files=0

# Find all example main.go files
while IFS= read -r file; do
    total_files=$((total_files + 1))
    echo "Processing: $file"
    
    # Create backup
    cp "$file" "$file.bak"
    
    # Check if context import exists
    if ! grep -q '"context"' "$file"; then
        echo "  → Adding context import"
        # Add context import after the first import line
        sed -i '' '/^import (/a\
	"context"
' "$file" 2>/dev/null || sed -i '/^import (/a\	"context"' "$file"
    fi
    
    # Fix duplicate context.Background() in all function calls first
    sed -i '' 's/(context\.Background(), context\.Background(),/(context.Background(),/g' "$file" 2>/dev/null || \
    sed -i 's/(context\.Background(), context\.Background(),/(context.Background(),/g' "$file"
    
    # Update Login calls - handle various patterns
    if grep -q 'client\.Login(' "$file" || grep -q 'restClient\.Login(' "$file"; then
        echo "  → Updating Login() calls"
        sed -i '' 's/client\.Login(/client.Login(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/client\.Login(/client.Login(context.Background(), /g' "$file"
        
        sed -i '' 's/restClient\.Login(/restClient.Login(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/restClient\.Login(/restClient.Login(context.Background(), /g' "$file"
        
        sed -i '' 's/hmcClient\.Login(/hmcClient.Login(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/hmcClient\.Login(/hmcClient.Login(context.Background(), /g' "$file"
        
        # Fix any duplicates that were just created
        sed -i '' 's/\.Login(context\.Background(), context\.Background(),/.Login(context.Background(),/g' "$file" 2>/dev/null || \
        sed -i 's/\.Login(context\.Background(), context\.Background(),/.Login(context.Background(),/g' "$file"
    fi
    
    # Update Logoff calls
    if grep -q '\.Logoff()' "$file"; then
        echo "  → Updating Logoff() calls"
        sed -i '' 's/client\.Logoff()/client.Logoff(context.Background())/g' "$file" 2>/dev/null || \
        sed -i 's/client\.Logoff()/client.Logoff(context.Background())/g' "$file"
        
        sed -i '' 's/restClient\.Logoff()/restClient.Logoff(context.Background())/g' "$file" 2>/dev/null || \
        sed -i 's/restClient\.Logoff()/restClient.Logoff(context.Background())/g' "$file"
        
        sed -i '' 's/hmcClient\.Logoff()/hmcClient.Logoff(context.Background())/g' "$file" 2>/dev/null || \
        sed -i 's/hmcClient\.Logoff()/hmcClient.Logoff(context.Background())/g' "$file"
    fi
    
    # Update DeleteJob calls
    if grep -q '\.DeleteJob(' "$file"; then
        echo "  → Updating DeleteJob() calls"
        sed -i '' 's/\.DeleteJob(/\.DeleteJob(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteJob(/\.DeleteJob(context.Background(), /g' "$file"
    fi
    
    # Update FetchJobResponse calls
    if grep -q '\.FetchJobResponse(' "$file"; then
        echo "  → Updating FetchJobResponse() calls"
        sed -i '' 's/\.FetchJobResponse(/\.FetchJobResponse(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.FetchJobResponse(/\.FetchJobResponse(context.Background(), /g' "$file"
    fi
    
    # Update GetManagedSystemByNameQuick calls
    if grep -q '\.GetManagedSystemByNameQuick(' "$file"; then
        echo "  → Updating GetManagedSystemByNameQuick() calls"
        sed -i '' 's/\.GetManagedSystemByNameQuick(/\.GetManagedSystemByNameQuick(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetManagedSystemByNameQuick(/\.GetManagedSystemByNameQuick(context.Background(), /g' "$file"
    fi
    
    # Update GetManagedSystem calls
    if grep -q '\.GetManagedSystem(' "$file"; then
        echo "  → Updating GetManagedSystem() calls"
        sed -i '' 's/\.GetManagedSystem(/\.GetManagedSystem(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetManagedSystem(/\.GetManagedSystem(context.Background(), /g' "$file"
    fi
    
    # Update GetSRIOVAdapters calls
    if grep -q '\.GetSRIOVAdapters(' "$file"; then
        echo "  → Updating GetSRIOVAdapters() calls"
        sed -i '' 's/\.GetSRIOVAdapters(/\.GetSRIOVAdapters(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetSRIOVAdapters(/\.GetSRIOVAdapters(context.Background(), /g' "$file"
    fi
    
    # Update CliRunner calls
    if grep -q '\.CliRunner(' "$file"; then
        echo "  → Updating CliRunner() calls"
        sed -i '' 's/\.CliRunner(/\.CliRunner(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CliRunner(/\.CliRunner(context.Background(), /g' "$file"
    fi
    
    # Update MountNFS calls
    if grep -q 'hmc\.MountNFS(' "$file"; then
        echo "  → Updating MountNFS() calls"
        sed -i '' 's/hmc\.MountNFS(/hmc.MountNFS(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/hmc\.MountNFS(/hmc.MountNFS(context.Background(), /g' "$file"
    fi
    
    # Update UnmountNFS calls
    if grep -q 'hmc\.UnmountNFS(' "$file"; then
        echo "  → Updating UnmountNFS() calls"
        sed -i '' 's/hmc\.UnmountNFS(/hmc.UnmountNFS(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/hmc\.UnmountNFS(/hmc.UnmountNFS(context.Background(), /g' "$file"
    fi
    
    # Update CloseVirtualTerminal calls
    if grep -q '\.CloseVirtualTerminal(' "$file"; then
        echo "  → Updating CloseVirtualTerminal() calls"
        sed -i '' 's/\.CloseVirtualTerminal(/\.CloseVirtualTerminal(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CloseVirtualTerminal(/\.CloseVirtualTerminal(context.Background(), /g' "$file"
    fi
    
    # Update GetManagedSystemByName calls
    if grep -q '\.GetManagedSystemByName(' "$file"; then
        echo "  → Updating GetManagedSystemByName() calls"
        sed -i '' 's/\.GetManagedSystemByName(/\.GetManagedSystemByName(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetManagedSystemByName(/\.GetManagedSystemByName(context.Background(), /g' "$file"
    fi
    
    # Update vios.go functions
    if grep -q '\.GetVirtualOpticalMedia(' "$file" || grep -q '\.GetVirtualOpticalMedias(' "$file"; then
        echo "  → Updating GetVirtualOpticalMedia() calls"
        sed -i '' 's/\.GetVirtualOpticalMedia(/\.GetVirtualOpticalMedia(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualOpticalMedia(/\.GetVirtualOpticalMedia(context.Background(), /g' "$file"
        
        sed -i '' 's/\.GetVirtualOpticalMedias(/\.GetVirtualOpticalMedias(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualOpticalMedias(/\.GetVirtualOpticalMedias(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.CreateVirtualDisk(' "$file" || grep -q '\.DeleteVirtualDisk(' "$file" || grep -q '\.ExtendVirtualDisk(' "$file"; then
        echo "  → Updating VirtualDisk functions"
        sed -i '' 's/\.CreateVirtualDisk(/\.CreateVirtualDisk(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CreateVirtualDisk(/\.CreateVirtualDisk(context.Background(), /g' "$file"
        
        sed -i '' 's/\.DeleteVirtualDisk(/\.DeleteVirtualDisk(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteVirtualDisk(/\.DeleteVirtualDisk(context.Background(), /g' "$file"
        
        sed -i '' 's/\.ExtendVirtualDisk(/\.ExtendVirtualDisk(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.ExtendVirtualDisk(/\.ExtendVirtualDisk(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.CreateVirtualOpticalMedia(' "$file" || grep -q '\.DeleteVirtualOpticalMedia(' "$file"; then
        echo "  → Updating VirtualOpticalMedia functions"
        sed -i '' 's/\.CreateVirtualOpticalMedia(/\.CreateVirtualOpticalMedia(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CreateVirtualOpticalMedia(/\.CreateVirtualOpticalMedia(context.Background(), /g' "$file"
        
        sed -i '' 's/\.DeleteVirtualOpticalMedia(/\.DeleteVirtualOpticalMedia(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteVirtualOpticalMedia(/\.DeleteVirtualOpticalMedia(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.CreateMediaRepository(' "$file" || grep -q '\.DeleteMediaRepository(' "$file" || grep -q '\.ChangeMediaRepository(' "$file"; then
        echo "  → Updating MediaRepository functions"
        sed -i '' 's/\.CreateMediaRepository(/\.CreateMediaRepository(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CreateMediaRepository(/\.CreateMediaRepository(context.Background(), /g' "$file"
        
        sed -i '' 's/\.DeleteMediaRepository(/\.DeleteMediaRepository(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteMediaRepository(/\.DeleteMediaRepository(context.Background(), /g' "$file"
        
        sed -i '' 's/\.ChangeMediaRepository(/\.ChangeMediaRepository(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.ChangeMediaRepository(/\.ChangeMediaRepository(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.GetViosSCSIMappings(' "$file"; then
        echo "  → Updating GetViosSCSIMappings() calls"
        sed -i '' 's/\.GetViosSCSIMappings(/\.GetViosSCSIMappings(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetViosSCSIMappings(/\.GetViosSCSIMappings(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.GetPhysicalFibreChannelPorts(' "$file" || grep -q '\.GetVirtualFibreChannelMaps(' "$file"; then
        echo "  → Updating FibreChannel functions"
        sed -i '' 's/\.GetPhysicalFibreChannelPorts(/\.GetPhysicalFibreChannelPorts(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetPhysicalFibreChannelPorts(/\.GetPhysicalFibreChannelPorts(context.Background(), /g' "$file"
        
        sed -i '' 's/\.GetVirtualFibreChannelMaps(/\.GetVirtualFibreChannelMaps(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualFibreChannelMaps(/\.GetVirtualFibreChannelMaps(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.AddVirtualOpticalMedia(' "$file" || grep -q '\.CreateVirtualOpticalMaps(' "$file"; then
        echo "  → Updating VirtualOptical mapping functions"
        sed -i '' 's/\.AddVirtualOpticalMedia(/\.AddVirtualOpticalMedia(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.AddVirtualOpticalMedia(/\.AddVirtualOpticalMedia(context.Background(), /g' "$file"
        
        sed -i '' 's/\.CreateVirtualOpticalMaps(/\.CreateVirtualOpticalMaps(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CreateVirtualOpticalMaps(/\.CreateVirtualOpticalMaps(context.Background(), /g' "$file"
    fi
    
    # Update SRIOV and Dedicated NIC functions
    if grep -q '\.GetDedicatedVirtualNICs(' "$file" || grep -q '\.GetSRIOVLogicalPorts(' "$file" || grep -q '\.DeleteSRIOVLogicalPorts(' "$file"; then
        echo "  → Updating SRIOV/Dedicated NIC functions"
        sed -i '' 's/\.GetDedicatedVirtualNICs(/\.GetDedicatedVirtualNICs(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetDedicatedVirtualNICs(/\.GetDedicatedVirtualNICs(context.Background(), /g' "$file"
        
        sed -i '' 's/\.GetSRIOVLogicalPorts(/\.GetSRIOVLogicalPorts(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetSRIOVLogicalPorts(/\.GetSRIOVLogicalPorts(context.Background(), /g' "$file"
        
        sed -i '' 's/\.DeleteSRIOVLogicalPorts(/\.DeleteSRIOVLogicalPorts(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteSRIOVLogicalPorts(/\.DeleteSRIOVLogicalPorts(context.Background(), /g' "$file"
    fi
    
    # Update network.go functions
    if grep -q '\.GetVirtualSwitchQuickAll(' "$file" || grep -q '\.GetVirtualSwitchQuick(' "$file" || grep -q '\.GetVirtualSwitches(' "$file"; then
        echo "  → Updating VirtualSwitch functions"
        sed -i '' 's/\.GetVirtualSwitchQuickAll(/\.GetVirtualSwitchQuickAll(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualSwitchQuickAll(/\.GetVirtualSwitchQuickAll(context.Background(), /g' "$file"
        
        sed -i '' 's/\.GetVirtualSwitchQuick(/\.GetVirtualSwitchQuick(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualSwitchQuick(/\.GetVirtualSwitchQuick(context.Background(), /g' "$file"
        
        sed -i '' 's/\.GetVirtualSwitches(/\.GetVirtualSwitches(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualSwitches(/\.GetVirtualSwitches(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.GetClientNetworkAdapters(' "$file" || grep -q '\.CreateClientNetworkAdapter(' "$file" || grep -q '\.DeleteClientNetworkAdapter(' "$file"; then
        echo "  → Updating ClientNetworkAdapter functions"
        sed -i '' 's/\.GetClientNetworkAdapters(/\.GetClientNetworkAdapters(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetClientNetworkAdapters(/\.GetClientNetworkAdapters(context.Background(), /g' "$file"
        
        sed -i '' 's/\.CreateClientNetworkAdapter(/\.CreateClientNetworkAdapter(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.CreateClientNetworkAdapter(/\.CreateClientNetworkAdapter(context.Background(), /g' "$file"
        
        sed -i '' 's/\.DeleteClientNetworkAdapter(/\.DeleteClientNetworkAdapter(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteClientNetworkAdapter(/\.DeleteClientNetworkAdapter(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.GetNetworkBootDevices(' "$file"; then
        echo "  → Updating GetNetworkBootDevices() calls"
        sed -i '' 's/\.GetNetworkBootDevices(/\.GetNetworkBootDevices(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetNetworkBootDevices(/\.GetNetworkBootDevices(context.Background(), /g' "$file"
    fi
    
    if grep -q '\.GetLocationCodeByMac(' "$file"; then
        echo "  → Updating GetLocationCodeByMac() calls"
        sed -i '' 's/\.GetLocationCodeByMac(/\.GetLocationCodeByMac(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetLocationCodeByMac(/\.GetLocationCodeByMac(context.Background(), /g' "$file"
    fi
    
    # Update GetManagedSystemQuickAll calls
    if grep -q '\.GetManagedSystemQuickAll(' "$file"; then
        echo "  → Updating GetManagedSystemQuickAll() calls"
        sed -i '' 's/\.GetManagedSystemQuickAll(/\.GetManagedSystemQuickAll(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetManagedSystemQuickAll(/\.GetManagedSystemQuickAll(context.Background(), /g' "$file"
    fi
    
    # Update GetLogicalPartitionsQuickAll calls
    if grep -q '\.GetLogicalPartitionsQuickAll(' "$file"; then
        echo "  → Updating GetLogicalPartitionsQuickAll() calls"
        sed -i '' 's/\.GetLogicalPartitionsQuickAll(/\.GetLogicalPartitionsQuickAll(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetLogicalPartitionsQuickAll(/\.GetLogicalPartitionsQuickAll(context.Background(), /g' "$file"
    fi
    
    # Update GetLogicalPartitionDetailed calls
    if grep -q '\.GetLogicalPartitionDetailed(' "$file"; then
        echo "  → Updating GetLogicalPartitionDetailed() calls"
        sed -i '' 's/\.GetLogicalPartitionDetailed(/\.GetLogicalPartitionDetailed(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetLogicalPartitionDetailed(/\.GetLogicalPartitionDetailed(context.Background(), /g' "$file"
    fi
    
    # Update SetPartitionBootString calls
    if grep -q '\.SetPartitionBootString(' "$file"; then
        echo "  → Updating SetPartitionBootString() calls"
        sed -i '' 's/\.SetPartitionBootString(/\.SetPartitionBootString(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.SetPartitionBootString(/\.SetPartitionBootString(context.Background(), /g' "$file"
    fi
    
    # Update GetLogicalPartitionByName calls
    if grep -q '\.GetLogicalPartitionByName(' "$file"; then
        echo "  → Updating GetLogicalPartitionByName() calls"
        sed -i '' 's/\.GetLogicalPartitionByName(/\.GetLogicalPartitionByName(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetLogicalPartitionByName(/\.GetLogicalPartitionByName(context.Background(), /g' "$file"
    fi
    
    # Update GetLogicalPartition calls
    if grep -q '\.GetLogicalPartition(' "$file"; then
        echo "  → Updating GetLogicalPartition() calls"
        sed -i '' 's/\.GetLogicalPartition(/\.GetLogicalPartition(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetLogicalPartition(/\.GetLogicalPartition(context.Background(), /g' "$file"
    fi
    
    # Update GetLogicalPartitionProfiles calls
    if grep -q '\.GetLogicalPartitionProfiles(' "$file"; then
        echo "  → Updating GetLogicalPartitionProfiles() calls"
        sed -i '' 's/\.GetLogicalPartitionProfiles(/\.GetLogicalPartitionProfiles(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetLogicalPartitionProfiles(/\.GetLogicalPartitionProfiles(context.Background(), /g' "$file"
    fi
    
    # Update SaveCurrentLparConfig calls
    if grep -q '\.SaveCurrentLparConfig(' "$file"; then
        echo "  → Updating SaveCurrentLparConfig() calls"
        sed -i '' 's/\.SaveCurrentLparConfig(/\.SaveCurrentLparConfig(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.SaveCurrentLparConfig(/\.SaveCurrentLparConfig(context.Background(), /g' "$file"
    fi
    
    # Update GetVirtualIOServersQuick calls
    if grep -q '\.GetVirtualIOServersQuick(' "$file"; then
        echo "  → Updating GetVirtualIOServersQuick() calls"
        sed -i '' 's/\.GetVirtualIOServersQuick(/\.GetVirtualIOServersQuick(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualIOServersQuick(/\.GetVirtualIOServersQuick(context.Background(), /g' "$file"
    fi
    
    # Update GetVolumeGroups calls
    if grep -q '\.GetVolumeGroups(' "$file"; then
        echo "  → Updating GetVolumeGroups() calls"
        sed -i '' 's/\.GetVolumeGroups(/\.GetVolumeGroups(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVolumeGroups(/\.GetVolumeGroups(context.Background(), /g' "$file"
    fi
    
    # Update DeleteVirtualOpticalMaps calls
    if grep -q '\.DeleteVirtualOpticalMaps(' "$file"; then
        echo "  → Updating DeleteVirtualOpticalMaps() calls"
        sed -i '' 's/\.DeleteVirtualOpticalMaps(/\.DeleteVirtualOpticalMaps(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.DeleteVirtualOpticalMaps(/\.DeleteVirtualOpticalMaps(context.Background(), /g' "$file"
    fi
    
    # Update GetActiveVIOSServers calls
    if grep -q '\.GetActiveVIOSServers(' "$file"; then
        echo "  → Updating GetActiveVIOSServers() calls"
        sed -i '' 's/\.GetActiveVIOSServers(/\.GetActiveVIOSServers(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetActiveVIOSServers(/\.GetActiveVIOSServers(context.Background(), /g' "$file"
    fi
    
    # Update GetVirtualIOServer calls
    if grep -q '\.GetVirtualIOServer(' "$file"; then
        echo "  → Updating GetVirtualIOServer() calls"
        sed -i '' 's/\.GetVirtualIOServer(/\.GetVirtualIOServer(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/\.GetVirtualIOServer(/\.GetVirtualIOServer(context.Background(), /g' "$file"
    fi
    
    # Update GetViosID calls (helper function)
    if grep -q 'hmc\.GetViosID(' "$file"; then
        echo "  → Updating GetViosID() calls"
        sed -i '' 's/hmc\.GetViosID(/hmc.GetViosID(context.Background(), /g' "$file" 2>/dev/null || \
        sed -i 's/hmc\.GetViosID(/hmc.GetViosID(context.Background(), /g' "$file"
    fi
    
    # Fix any duplicates that were created
    sed -i '' 's/(context\.Background(), context\.Background(),/(context.Background(),/g' "$file" 2>/dev/null || \
    sed -i 's/(context\.Background(), context\.Background(),/(context.Background(),/g' "$file"
    
    # Check if file was actually modified
    if ! diff -q "$file" "$file.bak" > /dev/null 2>&1; then
        updated_files=$((updated_files + 1))
        echo "  ✓ Updated"
    else
        echo "  - No changes needed"
    fi
    
    # Remove backup if no changes or keep it for reference
    # Uncomment the next line to remove backups
    # rm "$file.bak"
    
    echo ""
done < <(find examples -name "main.go" -type f)

echo "==================================="
echo "Migration Summary"
echo "==================================="
echo "Total files processed: $total_files"
echo "Files updated: $updated_files"
echo "Files unchanged: $((total_files - updated_files))"
echo ""
echo "Backup files created with .bak extension"
echo "Run 'find examples -name \"*.bak\" -delete' to remove backups"
echo ""
echo "Next steps:"
echo "1. Review changes: git diff"
echo "2. Test compilation: go build ./..."
echo "3. Run tests: go test ./..."
echo "4. Test examples manually"
echo ""
echo "Migration complete!"

# Made with Bob
