import type { PermissionDecision } from '@/types';

export async function sendPermissionDecision(decision: PermissionDecision): Promise<void> {
    const response = await fetch('/api/permissions/decision', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(decision),
    });

    if (!response.ok) {
        let message = `permission decision failed with ${response.status}`;
        try {
            const payload = await response.json();
            message = payload?.error?.message ?? message;
        } catch {
            // Keep the status-based fallback message.
        }
        throw new Error(message);
    }
}
