let socket, myId, currentState, activePuzzle;
let reconnectAttempts = 0;
let intentionalClose = false;
const maxReconnectAttempts = 5;
const $=id=>document.getElementById(id), log=$('log');
function add(text){const d=document.createElement('div');d.className='line';d.innerHTML=text;log.appendChild(d);log.scrollTop=log.scrollHeight}
function esc(s){return String(s).replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function render(s){
 if(!s)return; currentState=s;
 $('state').textContent=s.status.toUpperCase();
 $('objective').textContent=s.objective;
 $('fragments').textContent=`FRAGMENTS ${s.fragments_found}/${s.fragment_total} · PUZZLES ${s.puzzle_progress}/${s.puzzle_total}`;
 $('timer').textContent=`${String(Math.floor(s.remaining_time/60)).padStart(2,'0')}:${String(s.remaining_time%60).padStart(2,'0')}`;
 $('roomLabel').textContent=$('room').value.trim().toUpperCase();
 $('players').innerHTML=s.players.map(p=>`<div class="player ${p.ready?'ready':''}"><span>${esc(p.name)}</span><em>${p.ready?'READY':'WAIT'}</em></div>`).join('');
 $('nodes').innerHTML=s.nodes.map(n=>`<button class="node ${s.discovered.includes(n.id)?'seen':''} ${n.locked?'locked':''}" data-node="${n.id}" ${s.status!=='playing'||n.locked?'disabled':''}><span>${n.id}</span><small>${n.title}</small>${n.locked?'<i>LOCKED</i>':''}</button>`).join('');
 document.querySelectorAll('.node').forEach(b=>b.onclick=()=>handleNode(b.dataset.node));
 const me=s.players.find(p=>p.id===myId); $('ready').disabled=s.status!=='waiting'; $('ready').textContent=me?.ready?'UNREADY':'READY';
 $('codeForm').hidden=!(s.status==='playing' && s.core_ready && s.discovered.includes('SERVER-CORE'));
 $('briefing').hidden=s.status!=='waiting';
 if(s.status!=='playing') $('chatForm').querySelector('input').placeholder='transmit message...';
 if(activePuzzle){const p=s.puzzles.find(x=>x.id===activePuzzle);if(p?.solved) closePuzzle();}
 if(s.status==='won') showResult('ACCESS GRANTED','ESCAPE SUCCESSFUL');
 if(s.status==='lost') showResult('SYSTEM LOCKDOWN','TIME EXPIRED');
}
function handleNode(node){
 const puzzle=currentState?.puzzles?.find(p=>p.id===node);
 if(puzzle){openPuzzle(puzzle);return;}
 socket?.send(JSON.stringify({type:'explore_node',node}));
}
function openPuzzle(p){
 activePuzzle=p.id;
 $('puzzleTitle').textContent=p.title;
 $('puzzlePrompt').textContent=p.id==='PUZZLE-BLUE'?'2, 4, 6, 8, ?':p.id==='PUZZLE-RED'?'Middle switch position (1–3)?':'12 - 3 = ?';
 $('puzzleHint').textContent=p.prompt;
 $('puzzleAnswer').value=''; $('puzzleError').hidden=true; $('puzzle').hidden=false; $('puzzleAnswer').focus();
}
function closePuzzle(){activePuzzle=null;$('puzzle').hidden=true}
function showResult(title,sub){$('result').hidden=false;$('result').innerHTML=`<strong>${title}</strong><span>${sub}</span>`}
function connect(){
 const name=$('name').value.trim(),room=$('room').value.trim();
 if(!name||!room)return;
 myId=sessionStorage.getItem('ghost_protocol_player_id')||crypto.randomUUID();
 sessionStorage.setItem('ghost_protocol_player_id',myId);
 sessionStorage.setItem('ghost_protocol_name',name);
 sessionStorage.setItem('ghost_protocol_room',room);
 intentionalClose=false;
 $('status').textContent=reconnectAttempts?'RECONNECTING…':'CONNECTING…';
 socket=new WebSocket(`${location.protocol==='https:'?'wss':'ws'}://${location.host}/ws`);
 socket.onopen=()=>{
  reconnectAttempts=0;
  $('status').textContent='CONNECTED';$('join').hidden=true;$('game').hidden=false;
  socket.send(JSON.stringify({type:'join_room',player_id:myId,name,room_id:room}));
 };
 socket.onclose=()=>{
  $('status').textContent='OFFLINE';
  if(!intentionalClose&&reconnectAttempts<maxReconnectAttempts){
   reconnectAttempts++;
   add(`> CONNECTION LOST — reconnect attempt ${reconnectAttempts}/${maxReconnectAttempts}`);
   setTimeout(connect,Math.min(1000*2**(reconnectAttempts-1),8000));
  }else if(!intentionalClose){
   $('join').hidden=false;$('game').hidden=true;
   add('> RECONNECT WINDOW CLOSED — press CONNECT to try again');
  }
 };
 socket.onmessage=e=>{
  const m=JSON.parse(e.data);if(m.state)render(m.state);
  switch(m.type){
   case'chat':add(`<b>[${esc(m.name)}]</b> ${esc(m.message)}`);break;
   case'welcome':add(`> ${esc(m.message)}`);break;
   case'player_joined':add(`> ${esc(m.name)} joined the channel`);break;
   case'player_reconnected':add(`> ${esc(m.name)} RECONNECTED`);break;
   case'player_disconnected':add(`> ${esc(m.name)} disconnected — ${esc(m.message)}`);break;
   case'player_left':add(`> ${esc(m.name)} left the channel`);break;
   case'node_result':add(`<b>> ${esc(m.node)}</b> ${esc(m.message)}`);break;
   case'puzzle_result':closePuzzle();add(`<b>> ${esc(m.node)}</b> ${esc(m.message)}`);break;
   case'puzzle_error':$('puzzleError').textContent=m.message;$('puzzleError').hidden=false;add(`> ${esc(m.node)}: ${esc(m.message)}`);break;
   case'game_started':add('> SYSTEM LOCKDOWN STARTED — MOVE');break;
   case'game_won':showResult('ACCESS GRANTED','ESCAPE SUCCESSFUL');add('> CONNECTION TERMINATED — WELL DONE, OPERATIVES');break;
   case'game_lost':showResult('SYSTEM LOCKDOWN','TIME EXPIRED');add('> NETWORK SILENT');break;
   case'error':add(`> ERROR: ${esc(m.message)}`)
  }
 };
}
$('connect').onclick=connect;
const savedName=sessionStorage.getItem('ghost_protocol_name'),savedRoom=sessionStorage.getItem('ghost_protocol_room');
if(savedName)$('name').value=savedName;
if(savedRoom)$('room').value=savedRoom;
$('ready').onclick=()=>{const p=currentState?.players.find(p=>p.id===myId);socket?.send(JSON.stringify({type:'ready',ready:!p?.ready}))};
$('chatForm').onsubmit=e=>{e.preventDefault();const i=$('message'),m=i.value.trim();if(socket?.readyState===1&&m){socket.send(JSON.stringify({type:'chat',message:m}));i.value=''}};
$('codeForm').onsubmit=e=>{e.preventDefault();const i=$('code');if(i.value.trim()){socket.send(JSON.stringify({type:'submit_code',code:i.value.trim()}));i.value=''}};
$('puzzleForm').onsubmit=e=>{e.preventDefault();if(!activePuzzle)return;const answer=$('puzzleAnswer').value.trim();if(answer)socket?.send(JSON.stringify({type:'solve_puzzle',node:activePuzzle,answer}))};
$('puzzleClose').onclick=closePuzzle;
