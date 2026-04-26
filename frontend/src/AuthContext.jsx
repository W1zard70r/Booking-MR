import { createContext, useState, useContext, useEffect } from 'react';
import { apiFetch } from './api';
import { jwtDecode } from 'jwt-decode';

const AuthContext = createContext();

export const AuthProvider = ({ children }) => {
    const [token, setToken] = useState(localStorage.getItem('token'));
    const [role, setRole] = useState(null);
    const [loading, setLoading] = useState(true);

    // При первой загрузке достаем роль из сохраненного токена
    useEffect(() => {
        if (token) {
            try {
                const decoded = jwtDecode(token);
                setRole(decoded.role);
            } catch (err) {
                console.error("Ошибка чтения токена", err);
                logout();
            }
        }
        setLoading(false);
    }, [token]);

    const login = async (email, password) => {
        try {
            const data = await apiFetch('/login', {
                method: 'POST',
                body: JSON.stringify({ email, password })
            });

            const decoded = jwtDecode(data.token); // Достаем role из JWT

            setToken(data.token);
            setRole(decoded.role);
            localStorage.setItem('token', data.token);

            return decoded.role; // Возвращаем роль для редиректа в Login.jsx
        } catch (err) {
            alert("Ошибка входа: " + err.message);
            throw err;
        }
    };

    const register = async (email, password) => {
        try {
            await apiFetch('/register', {
                method: 'POST',
                body: JSON.stringify({ email, password, role: 'user' }) // По ТЗ регаем юзеров
            });
            return await login(email, password); // Логинимся и возвращаем роль
        } catch (err) {
            alert("Ошибка регистрации: " + err.message);
            throw err;
        }
    };

    const logout = () => {
        setToken(null);
        setRole(null);
        localStorage.removeItem('token');
    };

    return (
        <AuthContext.Provider value={{ token, role, login, register, logout, loading }}>
            {children}
        </AuthContext.Provider>
    );
};

export const useAuth = () => useContext(AuthContext);