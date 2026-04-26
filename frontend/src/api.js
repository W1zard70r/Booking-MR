const BASE_URL = '/api';

export const apiFetch = async (endpoint, options = {}) => {
    const token = localStorage.getItem('token');

    const headers = {
        'Content-Type': 'application/json',
        ...(token && { 'Authorization': `Bearer ${token}` }),
        ...options.headers,
    };

    const response = await fetch(`${BASE_URL}${endpoint}`, {
        ...options,
        headers,
    });

    // Обработка ошибок (коды 4xx и 5xx)
    if (!response.ok) {
        // Если бэкенд ответил 401 Unauthorized (невалидный, истекший или поддельный токен)
        if (response.status === 401) {
            console.warn("Токен недействителен. Выполняем выход...");
            localStorage.removeItem('token'); // Удаляем плохой токен

            // Чтобы избежать бесконечного цикла, делаем редирект, 
            // только если мы еще не на странице логина
            if (window.location.pathname !== '/login') {
                window.location.href = '/login';
            }
        }

        // Пытаемся достать сообщение об ошибке от бэкенда (как прописано в твоем API)
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData?.error?.message || 'API Error');
    }

    return response.json();
};