(self.webpackChunk_N_E=self.webpackChunk_N_E||[]).push([[7332],{6131:function(e,t,n){(window.__NEXT_P=window.__NEXT_P||[]).push(["/federation/actions",function(){return n(63646)}])},63646:function(e,t,n){"use strict";n.r(t),n.d(t,{default:function(){return v}});var r=n(34051),i=n.n(r),a=n(85893),o=n(67294),s=n(84485),c=n(96003),u=n(58091),l=n(58827),f=n(2766);function d(e,t,n,r,i,a,o){try{var s=e[a](o),c=s.value}catch(u){return void n(u)}s.done?t(c):Promise.resolve(c).then(r,i)}var h=s.Z.Title,p=s.Z.Paragraph;function v(){var e=(0,o.useState)([]),t=e[0],n=e[1],r=(0,o.useState)(0),s=r[0],v=r[1],E=(0,o.useState)(0),w=E[0],m=E[1],_=function(){var e,t=(e=i().mark((function e(){var t,r,a,o,s;return i().wrap((function(e){for(;;)switch(e.prev=e.next){case 0:return e.prev=0,t=50*w,r="".concat(l.op,"?offset=").concat(t,"&limit=").concat(50),e.next=6,(0,l.rQ)(r,{auth:!0});case 6:a=e.sent,o=a.results,s=a.total,v(s),(0,f.Qr)(o)?n([]):n(o),e.next=15;break;case 12:e.prev=12,e.t0=e.catch(0),console.log("==== error",e.t0);case 15:case"end":return e.stop()}}),e,null,[[0,12]])})),function(){var t=this,n=arguments;return new Promise((function(r,i){var a=e.apply(t,n);function o(e){d(a,r,i,o,s,"next",e)}function s(e){d(a,r,i,o,s,"throw",e)}o(void 0)}))});return function(){return t.apply(this,arguments)}}();(0,o.useEffect)((function(){_()}),[w]);var g,x,y=[{title:"Action",dataIndex:"type",key:"type",width:50,render:function(e,t){var n,r;switch(t.type){case"FEDIVERSE_ENGAGEMENT_REPOST":n="/img/repost.svg",r="Share";break;case"FEDIVERSE_ENGAGEMENT_LIKE":n="/img/like.svg",r="Like";break;case"FEDIVERSE_ENGAGEMENT_FOLLOW":n="/img/follow.svg",r="Follow";break;default:n=""}return(0,a.jsxs)("div",{style:{width:"100%",height:"100%",display:"flex",alignItems:"center",justifyContent:"center",flexDirection:"column"},children:[(0,a.jsx)("img",{src:n,width:"70%",alt:r,title:r}),(0,a.jsx)("div",{style:{fontSize:"0.7rem"},children:r})]})}},{title:"From",dataIndex:"actorIRI",key:"from",render:function(e,t){return(0,a.jsx)("a",{href:t.actorIRI,children:t.actorIRI})}},{title:"When",dataIndex:"timestamp",key:"timestamp",render:function(e,t){var n=new Date(t.timestamp);return(0,u.Z)(n,"P pp")}}];return(0,a.jsxs)("div",{children:[(0,a.jsx)(h,{level:3,children:"Fediverse Actions"}),(0,a.jsx)(p,{children:"Below is a list of actions that were taken by others in response to your posts as well as people who requested to follow you."}),(g=t,x=y,(0,a.jsx)(c.Z,{dataSource:g,columns:x,size:"small",rowKey:function(e){return e.iri},pagination:{pageSize:50,hideOnSinglePage:!0,showSizeChanger:!1,total:s},onChange:function(e){var t=e.current;m(t)}}))]})}}},function(e){e.O(0,[1741,6003,8091,9774,2888,179],(function(){return t=6131,e(e.s=t);var t}));var t=e.O();_N_E=t}]);