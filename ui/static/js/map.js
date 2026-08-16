const places = JSON.parse(document.getElementById('map-data').textContent);

const map = L.map('map').setView([40.505, -25], 3);

L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution:
        '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    maxZoom: 18,
}).addTo(map);

const popup = (place) => {
    const holder = document.createElement('div');
    holder.className = 'place';

    const name = document.createElement('h4');
    name.textContent = place.name;
    holder.appendChild(name);

    for (const person of place.people) {
        const link = document.createElement('a');
        link.href = '/person/' + person.id;
        link.textContent = person.name;
        holder.appendChild(link);
    }

    return holder;
};

for (const place of places) {
    L.circleMarker([place.lat, place.lng], {
        radius: 8 * Math.sqrt(place.people.length),
        color: '#0d6efd',
        fillColor: '#0d6efd',
        fillOpacity: 0.4,
        weight: 1,
    })
        .bindPopup(popup(place))
        .addTo(map);
}
