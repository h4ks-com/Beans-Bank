async function getAuthenticatedUser() {
    try {
        const response = await fetch('/browser/wallet', {
            credentials: 'same-origin'
        });

        if (response.ok) {
            const data = await response.json();
            return { username: data.username };
        }
        return null;
    } catch (error) {
        console.error('Failed to get authenticated user:', error);
        return null;
    }
}
