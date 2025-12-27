import { writable } from 'svelte/store';

export const files = writable([]);
export const targetPath = writable("");
export const isScanning = writable(false);