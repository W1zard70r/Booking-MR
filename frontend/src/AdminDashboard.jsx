import { useState, useEffect } from 'react';
import { apiFetch } from './api';
import { useAuth } from './AuthContext';

const DAYS_OF_WEEK = [
    { id: 1, label: 'Пн' }, { id: 2, label: 'Вт' }, { id: 3, label: 'Ср' },
    { id: 4, label: 'Чт' }, { id: 5, label: 'Пт' }, { id: 6, label: 'Сб' }, { id: 7, label: 'Вс' }
];

export default function AdminDashboard() {
    const { logout } = useAuth();
    const [rooms, setRooms] = useState([]);
    const [newRoomName, setNewRoomName] = useState('');
    const [activeScheduleRoom, setActiveScheduleRoom] = useState(null); // ID комнаты, для которой открыта форма расписания

    // Состояние формы расписания
    const [selectedDays, setSelectedDays] = useState([1, 2, 3, 4, 5]);
    const [startTime, setStartTime] = useState('09:00');
    const [endTime, setEndTime] = useState('18:00');

    const loadData = async () => {
        try {
            const roomRes = await apiFetch('/rooms/list');
            setRooms(roomRes.rooms || []);
        } catch (e) { console.error(e); }
    };

    useEffect(() => { loadData(); }, []);

    const handleCreateRoom = async (e) => {
        e.preventDefault();
        try {
            await apiFetch('/rooms/create', {
                method: 'POST',
                body: JSON.stringify({ name: newRoomName, capacity: 10 })
            });
            setNewRoomName('');
            loadData();
        } catch (e) { alert(e.message); }
    };

    const handleCreateSchedule = async (roomId) => {
        if (selectedDays.length === 0) return alert("Выберите хотя бы один день недели");
        try {
            await apiFetch(`/rooms/${roomId}/schedule/create`, {
                method: 'POST',
                body: JSON.stringify({
                    daysOfWeek: selectedDays,
                    startTime,
                    endTime
                })
            });
            alert("Расписание успешно создано! Слоты генерируются.");
            setActiveScheduleRoom(null); // закрываем форму
        } catch (e) { alert(e.message); }
    };

    const toggleDay = (dayId) => {
        setSelectedDays(prev =>
            prev.includes(dayId) ? prev.filter(d => d !== dayId) : [...prev, dayId].sort()
        );
    };

    return (
        <div className="p-8 max-w-4xl mx-auto">
            <div className="flex justify-between items-center mb-8">
                <h1 className="text-3xl font-bold">Панель Администратора</h1>
                <button onClick={logout} className="text-gray-600 hover:underline">Выйти</button>
            </div>

            <div className="bg-white p-6 rounded shadow">
                <h2 className="text-xl font-semibold mb-4">Управление переговорками</h2>
                <form onSubmit={handleCreateRoom} className="flex gap-2 mb-6">
                    <input
                        value={newRoomName} onChange={(e) => setNewRoomName(e.target.value)}
                        placeholder="Название новой переговорки" className="border p-2 rounded flex-1 outline-none focus:ring-2 focus:ring-blue-500" required
                    />
                    <button className="bg-green-600 hover:bg-green-700 transition text-white px-6 py-2 rounded">Создать</button>
                </form>

                <div className="space-y-4">
                    {rooms.map(r => (
                        <div key={r.id} className="border p-4 rounded bg-gray-50">
                            <div className="flex justify-between items-center">
                                <span className="font-medium text-lg">{r.name}</span>
                                <button
                                    onClick={() => setActiveScheduleRoom(activeScheduleRoom === r.id ? null : r.id)}
                                    className="text-sm bg-blue-100 text-blue-700 px-4 py-2 rounded hover:bg-blue-200 transition font-medium"
                                >
                                    {activeScheduleRoom === r.id ? 'Скрыть расписание' : '+ Добавить расписание'}
                                </button>
                            </div>

                            {/* Форма создания расписания */}
                            {activeScheduleRoom === r.id && (
                                <div className="mt-4 p-4 border-t border-gray-200 bg-white rounded">
                                    <h3 className="text-sm font-semibold mb-3 text-gray-700">Настройка расписания</h3>

                                    <div className="mb-4">
                                        <label className="block text-xs text-gray-500 mb-2 uppercase tracking-wide">Дни недели</label>
                                        <div className="flex flex-wrap gap-2">
                                            {DAYS_OF_WEEK.map(day => (
                                                <button
                                                    key={day.id}
                                                    onClick={() => toggleDay(day.id)}
                                                    className={`px-3 py-1 rounded-full text-sm transition ${selectedDays.includes(day.id) ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
                                                        }`}
                                                >
                                                    {day.label}
                                                </button>
                                            ))}
                                        </div>
                                    </div>

                                    <div className="flex gap-4 mb-4">
                                        <div className="flex-1">
                                            <label className="block text-xs text-gray-500 mb-1 uppercase tracking-wide">Время начала</label>
                                            <input type="time" value={startTime} onChange={e => setStartTime(e.target.value)} className="border p-2 rounded w-full outline-none focus:ring-2 focus:ring-blue-500" />
                                        </div>
                                        <div className="flex-1">
                                            <label className="block text-xs text-gray-500 mb-1 uppercase tracking-wide">Время конца</label>
                                            <input type="time" value={endTime} onChange={e => setEndTime(e.target.value)} className="border p-2 rounded w-full outline-none focus:ring-2 focus:ring-blue-500" />
                                        </div>
                                    </div>

                                    <button
                                        onClick={() => handleCreateSchedule(r.id)}
                                        className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm w-full transition"
                                    >
                                        Сгенерировать слоты
                                    </button>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}