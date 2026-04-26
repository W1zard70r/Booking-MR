import { useState, useEffect } from 'react';
import { apiFetch } from './api';
import { useAuth } from './AuthContext';
import { ChevronLeft, ChevronRight } from 'lucide-react';

const getLocalYYYYMMDD = (date) => {
    const d = new Date(date);
    d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
    return d.toISOString().split('T')[0];
};

export default function UserDashboard() {
    const { logout } = useAuth();
    const [rooms, setRooms] = useState([]);
    const [selectedRoom, setSelectedRoom] = useState('');

    const [weekOffset, setWeekOffset] = useState(0);
    const [slotsByDay, setSlotsByDay] = useState({});
    const [myBookings, setMyBookings] = useState([]);

    const loadInitialData = async () => {
        try {
            const roomRes = await apiFetch('/rooms/list');
            setRooms(roomRes.rooms || []);
            if (roomRes.rooms?.length > 0) setSelectedRoom(roomRes.rooms[0].id);
            loadMyBookings();
        } catch (e) { console.error(e); }
    };

    const loadMyBookings = async () => {
        try {
            const res = await apiFetch('/bookings/my');
            setMyBookings(res.bookings || []);
        } catch (e) { console.error(e); }
    };

    const getDaysOfWeek = () => {
        const today = new Date();
        today.setDate(today.getDate() + (weekOffset * 7));
        return Array.from({ length: 7 }, (_, i) => {
            const d = new Date(today);
            d.setDate(d.getDate() + i);
            return getLocalYYYYMMDD(d);
        });
    };

    const weekDays = getDaysOfWeek();

    const loadWeekSlots = async () => {
        if (!selectedRoom) return;
        const results = await Promise.all(weekDays.map(async (date) => {
            try {
                const res = await apiFetch(`/rooms/${selectedRoom}/slots/list?date=${date}`);
                return { date, slots: res.slots || [] };
            } catch (e) {
                return { date, slots: [] };
            }
        }));

        const map = {};
        results.forEach(r => map[r.date] = r.slots);
        setSlotsByDay(map);
    };

    useEffect(() => { loadInitialData(); }, []);
    useEffect(() => { loadWeekSlots(); }, [selectedRoom, weekOffset]);

    const handleBook = async (slotId) => {
        try {
            await apiFetch('/bookings/create', { method: 'POST', body: JSON.stringify({ slotId }) });
            loadWeekSlots();
            loadMyBookings();
        } catch (e) { alert(e.message); }
    };

    const handleCancel = async (bookingId) => {
        if (!confirm('Отменить бронирование?')) return;
        try {
            await apiFetch(`/bookings/${bookingId}/cancel`, { method: 'POST' });
            loadWeekSlots();
            loadMyBookings();
        } catch (e) { alert(e.message); }
    };

    // --------------------------------------------------------
    // ГРУППИРОВКА БРОНИРОВАНИЙ ПО ДНЯМ ДЛЯ "КАЛЕНДАРЯ СПРАВА"
    // --------------------------------------------------------
    const groupedBookings = myBookings.reduce((acc, b) => {
        if (!b.slotStart) {
            // Если бэкенд не пересобрали, кидаем в отдельную группу
            const key = "Требуется пересборка Бэкенда (docker-compose build)";
            if (!acc[key]) acc[key] = [];
            acc[key].push(b);
            return acc;
        }

        // Получаем красивую дату (например: "Вторник, 28 апреля")
        const dateObj = new Date(b.slotStart);
        const dateKey = dateObj.toLocaleDateString('ru-RU', { weekday: 'long', day: 'numeric', month: 'long' });

        if (!acc[dateKey]) acc[dateKey] = [];
        acc[dateKey].push(b);
        return acc;
    }, {});

    return (
        <div className="p-8 max-w-7xl mx-auto">
            <div className="flex justify-between items-center mb-8">
                <h1 className="text-3xl font-bold text-gray-800">Бронирование</h1>
                <button onClick={logout} className="text-gray-600 hover:text-red-600 hover:underline">Выйти</button>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">

                {/* ЛЕВАЯ ЧАСТЬ: СЕТКА СЛОТОВ */}
                <div className="lg:col-span-2 bg-white p-6 rounded shadow">
                    <div className="flex flex-col md:flex-row justify-between items-center mb-6 gap-4">
                        <select
                            value={selectedRoom} onChange={(e) => setSelectedRoom(e.target.value)}
                            className="border border-gray-300 p-2 rounded w-full md:w-1/2 outline-none focus:ring-2 focus:ring-blue-500"
                        >
                            {rooms.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
                        </select>

                        <div className="flex items-center gap-4 bg-gray-50 border border-gray-200 rounded p-1">
                            <button onClick={() => setWeekOffset(w => w - 1)} className="p-1 text-gray-600 hover:bg-gray-200 rounded">
                                <ChevronLeft size={20} />
                            </button>
                            <span className="text-sm font-medium w-36 text-center text-gray-700">
                                {new Date(weekDays[0]).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })} - <br />
                                {new Date(weekDays[6]).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })}
                            </span>
                            <button onClick={() => setWeekOffset(w => w + 1)} className="p-1 text-gray-600 hover:bg-gray-200 rounded">
                                <ChevronRight size={20} />
                            </button>
                        </div>
                    </div>

                    <div className="grid grid-cols-2 md:grid-cols-7 gap-4">
                        {weekDays.map(dateStr => {
                            const dateObj = new Date(dateStr);
                            const dayName = dateObj.toLocaleDateString('ru-RU', { weekday: 'short' });
                            const dayNum = dateObj.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
                            const slots = slotsByDay[dateStr] || [];

                            return (
                                <div key={dateStr} className="flex flex-col border border-gray-200 rounded bg-gray-50 h-[450px] overflow-hidden">
                                    <div className="bg-gray-200 text-center py-2 text-sm border-b border-gray-300">
                                        <div className="capitalize font-semibold text-gray-700">{dayName}</div>
                                        <div className="text-xs text-gray-500">{dayNum}</div>
                                    </div>
                                    <div className="p-2 space-y-2 overflow-y-auto flex-1 custom-scrollbar">
                                        {slots.length === 0 ? (
                                            <div className="text-xs text-center text-gray-400 mt-4">Нет слотов</div>
                                        ) : (
                                            slots.map(s => {
                                                const time = new Date(s.start).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
                                                return (
                                                    <button
                                                        key={s.id} onClick={() => handleBook(s.id)}
                                                        className="w-full bg-white text-blue-700 font-medium border border-blue-200 py-1.5 text-sm rounded hover:bg-blue-600 hover:text-white transition shadow-sm"
                                                    >
                                                        {time}
                                                    </button>
                                                )
                                            })
                                        )}
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                </div>

                {/* ПРАВАЯ ЧАСТЬ: ВЕРТИКАЛЬНЫЙ КАЛЕНДАРЬ БРОНИРОВАНИЙ */}
                <div className="bg-white p-6 rounded shadow sticky top-8">
                    <h2 className="text-xl font-semibold mb-4 border-b pb-2 text-gray-800">Расписание броней</h2>

                    <div className="max-h-[600px] overflow-y-auto pr-2 custom-scrollbar">
                        {Object.keys(groupedBookings).length === 0 ? (
                            <p className="text-gray-500 text-sm text-center mt-4">У вас пока нет активных броней.</p>
                        ) : (
                            Object.entries(groupedBookings).map(([dateLabel, bookings]) => (
                                <div key={dateLabel} className="mb-6 last:mb-0">
                                    {/* Заголовок Дня */}
                                    <div className="sticky top-0 bg-white pb-2 z-10">
                                        <h3 className="text-sm font-bold text-gray-700 uppercase tracking-wide border-l-4 border-blue-500 pl-2 capitalize">
                                            {dateLabel}
                                        </h3>
                                    </div>

                                    {/* Список броней в этот день */}
                                    <div className="space-y-3 mt-2 pl-3 border-l-2 border-gray-100">
                                        {bookings.map(b => {
                                            const timeStart = b.slotStart ? new Date(b.slotStart).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : '';
                                            const timeEnd = b.slotEnd ? new Date(b.slotEnd).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : '';

                                            return (
                                                <div key={b.id} className="bg-gray-50 border border-gray-200 p-3 rounded shadow-sm flex flex-col gap-2">

                                                    <div className="flex justify-between items-start">
                                                        <span className="font-semibold text-gray-800">
                                                            {b.roomName || "ID слота: " + b.slotId}
                                                        </span>
                                                        <span className={`text-[10px] font-bold px-2 py-0.5 rounded-full uppercase ${b.status === 'cancelled' ? 'bg-gray-200 text-gray-500' : 'bg-green-100 text-green-700'
                                                            }`}>
                                                            {b.status === 'cancelled' ? 'Отменена' : 'Активна'}
                                                        </span>
                                                    </div>

                                                    {timeStart && (
                                                        <div className="text-blue-700 font-medium text-sm flex items-center gap-1">
                                                            <span>🕒</span> {timeStart} - {timeEnd}
                                                        </div>
                                                    )}

                                                    {b.status === 'active' && (
                                                        <button
                                                            onClick={() => handleCancel(b.id)}
                                                            className="mt-2 text-xs font-medium text-red-500 hover:text-red-700 hover:underline text-left w-fit"
                                                        >
                                                            Отменить бронь
                                                        </button>
                                                    )}
                                                </div>
                                            )
                                        })}
                                    </div>
                                </div>
                            ))
                        )}
                    </div>
                </div>

            </div>
        </div>
    );
}